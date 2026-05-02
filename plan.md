# Home Router Software — Implementation Plan

## Context

Turkcell Superonline'ın ISP modemleri bufferbloat sorununa neden oluyor ve 1 Gbps bağlantıda SQM/QoS desteği sunmuyor. Mevcut ZTE modem yerine Intel i5 3470 tabanlı özel donanım üzerine sıfırdan router yazılımı geliştirilecek. Hedef: PPPoE WAN bağlantısı, nftables firewall, WireGuard VPN, Samba NAS ve web dashboard'u tek bir Python uygulamasında birleştirmek.

## Current State

- Proje dizini boş — sıfırdan (greenfield) geliştirme
- Donanım hazır: 2x Gigabit NIC, RAID-1 depolama, Ubuntu 24.04

## What We're NOT Doing

- IPv6 desteği (v1 kapsamı dışı — `ip6tables -P FORWARD DROP` ile kapatılacak)
- Wi-Fi yönetimi (kullanıcı ayrı AP'ler kullanıyor)
- DHCP/DNS sunucu yazılımı (AdGuard Home'a devredilecek)
- Veritabanı (tüm config YAML/JSON dosyalarında)
- Frontend framework (React/Vue/Svelte yok — pure vanilla)
- Çoklu ISP / failover (tek PPPoE bağlantı)
- Konteyner/Docker desteği

---

## Mimari Kararlar

### 1. Privilege Separation (Ayrıcalık Ayrımı)

İki ayrı systemd servisi:

```
┌─────────────────────────────┐     ┌──────────────────────────────┐
│  home-router-web.service    │     │  home-router-agent.service   │
│  User: homerouter           │────▶│  User: root                  │
│  FastAPI + Uvicorn          │ UDS │  Unix Socket IPC             │
│  Port 8443 (LAN only)      │     │  Op Whitelist Dispatcher     │
└─────────────────────────────┘     └──────────────────────────────┘
        │                                      │
        ▼                                      ▼
   Web Dashboard                    nftables, pppd, wg, tc,
   REST API, WebSocket              ip rule/route, smartctl
```

- **Web process** (unprivileged) asla `subprocess` ile root komut çalıştırmaz
- **Agent process** (root) strict op whitelist ile yalnızca bilinen işlemleri yürütür
- IPC: Unix domain socket (`/run/home-router/agent.sock`)

### 2. Atomic Network Changes

```python
async with AtomicChange(service="firewall") as txn:
    txn.snapshot()          # mevcut nftables ruleset'i kaydet
    txn.validate(new_rules) # nft -c -f ile dry-run
    txn.apply(new_rules)    # nft -f ile uygula
    # hata olursa __aexit__ otomatik rollback yapar
```

Agent'ta 30 saniyelik watchdog: apply sonrası web'den onay gelmezse otomatik rollback.

### 3. VPN Policy Routing

```
nftables fwmark (kaynak IP'ye göre) → ip rule fwmark X lookup table_wgN → per-table default route
ct mark ile reply paketlerde fwmark korunur
```

### 4. AdGuard Home Integration

AdGuard Home tek DHCP/DNS otoritesi. Router uygulaması kendi DHCP sunucusu çalıştırmaz — tüm DHCP/DNS yönetimi AGH REST API üzerinden proxy edilir.

---

## Dizin Yapısı

```
/opt/home-router/
├── home_router/                  # Ana Python paketi
│   ├── __init__.py
│   ├── main.py                   # FastAPI app factory
│   ├── config/
│   │   ├── __init__.py
│   │   ├── manager.py            # YAML load/save, Fernet encryption
│   │   ├── schema.py             # Pydantic config modelleri
│   │   └── defaults.py           # Varsayılan değerler
│   ├── agent/
│   │   ├── __init__.py
│   │   ├── server.py             # Root agent — UDS dinleyici, op dispatcher
│   │   ├── client.py             # Web'den agent'a IPC istemcisi
│   │   ├── operations.py         # İzin verilen op tanımları
│   │   └── watchdog.py           # Rollback watchdog timer
│   ├── api/
│   │   ├── __init__.py
│   │   ├── deps.py               # Dependency injection (auth, config)
│   │   ├── middleware.py          # CORS, rate limiting, CSRF
│   │   └── routes/
│   │       ├── __init__.py
│   │       ├── auth.py           # Login/logout/session
│   │       ├── dashboard.py      # Sistem istatistikleri
│   │       ├── network.py        # Interface bilgileri
│   │       ├── pppoe.py          # WAN bağlantı yönetimi
│   │       ├── firewall.py       # nftables kuralları
│   │       ├── adguard.py        # AGH proxy endpoint'leri
│   │       ├── qos.py            # SQM/QoS profilleri
│   │       ├── vpn.py            # WireGuard tünelleri + cihaz ataması
│   │       ├── nas.py            # Samba paylaşımları
│   │       ├── storage.py        # RAID durumu, disk sağlığı
│   │       └── ws.py             # WebSocket real-time stats
│   ├── services/
│   │   ├── __init__.py
│   │   ├── pppoe_service.py      # pppd yönetimi
│   │   ├── firewall_service.py   # nftables ruleset oluşturma + uygulama
│   │   ├── adguard_service.py    # AGH REST API istemcisi
│   │   ├── qos_service.py        # tc + CAKE qdisc yönetimi
│   │   ├── vpn_service.py        # WireGuard tunnel + policy routing
│   │   ├── nas_service.py        # Samba config + M3U parser
│   │   ├── storage_service.py    # mdadm + smartctl
│   │   ├── monitor_service.py    # Sistem istatistikleri toplayıcı
│   │   └── backup_service.py     # Config export/import
│   ├── templates/                # Jinja2 şablonları (config dosyaları için)
│   │   ├── nftables/
│   │   │   └── main.nft.j2       # Ana nftables ruleset
│   │   ├── pppoe/
│   │   │   ├── peer.j2           # /etc/ppp/peers/wan
│   │   │   └── options.j2        # pppd seçenekleri
│   │   ├── wireguard/
│   │   │   └── wg.conf.j2        # WireGuard interface config
│   │   └── samba/
│   │       └── smb.conf.j2       # Samba paylaşım config
│   └── utils/
│       ├── __init__.py
│       ├── atomic.py             # AtomicChange context manager
│       ├── crypto.py             # Fernet key yönetimi, WG keypair
│       ├── netlink.py            # Interface bilgisi okuma
│       ├── validators.py         # IP, CIDR, port doğrulama
│       └── process.py            # Güvenli subprocess wrapper
├── frontend/
│   ├── index.html                # SPA shell
│   ├── css/
│   │   ├── reset.css
│   │   ├── variables.css         # CSS custom properties (tema)
│   │   ├── layout.css
│   │   ├── components.css
│   │   └── pages.css
│   ├── js/
│   │   ├── app.js                # Router, state management
│   │   ├── api.js                # Fetch wrapper + auth
│   │   ├── ws.js                 # WebSocket istemcisi
│   │   ├── components/
│   │   │   ├── sidebar.js
│   │   │   ├── toast.js
│   │   │   ├── modal.js
│   │   │   ├── chart.js          # Canvas-based grafikler (no lib)
│   │   │   └── drag-drop.js      # VPN cihaz sürükle-bırak
│   │   └── pages/
│   │       ├── dashboard.js
│   │       ├── network.js
│   │       ├── firewall.js
│   │       ├── vpn.js
│   │       ├── dns.js
│   │       ├── qos.js
│   │       ├── nas.js
│   │       ├── storage.js
│   │       └── settings.js
│   └── assets/
│       ├── icons/                # SVG ikonlar
│       └── fonts/                # Self-hosted font (Inter veya system)
├── config/
│   ├── router.yaml               # Ana yapılandırma
│   ├── firewall.yaml             # nftables kuralları
│   ├── vpn.yaml                  # WireGuard tünelleri + cihaz atamaları
│   ├── qos.yaml                  # SQM profilleri
│   ├── nas.yaml                  # Samba paylaşımları
│   └── .credentials.enc          # Fernet ile şifrelenmiş PPPoE credentials
├── systemd/
│   ├── home-router.target        # Orchestration target
│   ├── home-router-agent.service # Root agent
│   └── home-router-web.service   # Web UI + API
├── scripts/
│   ├── install.sh                # Tam kurulum scripti
│   ├── setup-interfaces.sh       # udev kuralları + NIC isimlendirme
│   ├── factory-reset.sh          # Fabrika ayarlarına dönüş
│   └── backup.sh                 # Cron backup scripti
├── tests/
│   ├── unit/                     # pytest unit testleri
│   ├── integration/              # netns tabanlı network testleri
│   └── conftest.py
├── pyproject.toml                # Proje metadata + bağımlılıklar
├── requirements.txt              # Pinlenmiş bağımlılıklar
└── README.md
```

---

## Config Schema (router.yaml)

```yaml
system:
  hostname: "home-router"
  timezone: "Europe/Istanbul"
  admin_password_hash: "$2b$12$..."      # bcrypt
  session_secret: "..."                   # 32-byte hex
  web_port: 8443
  web_bind: "10.0.0.1"                   # Sadece LAN

interfaces:
  wan:
    device: "enp3s0"                      # udev rule ile sabitlenmiş
    type: "pppoe"
    mtu: 1492
  lan:
    device: "enp0s25"
    address: "10.0.0.1/24"
    mtu: 1500

pppoe:
  username: "..."                         # .credentials.enc'den okunur
  password: "..."
  mtu: 1492
  mru: 1492
  lcp_echo_interval: 10
  lcp_echo_failure: 3
  persist: true
  holdoff: 5

firewall:
  default_policy: "drop"                  # WAN input/forward
  port_forwards: []
  rate_limits:
    ssh: "3/minute"
    web: "30/minute"

qos:
  enabled: true
  profile: "cake"                         # cake | fq_codel | none
  upload_kbps: 40000                      # ISP upload
  download_kbps: 950000                   # ISP download (1Gbps - overhead)
  congestion_control: "bbr"               # bbr | cubic
  per_device_limits: {}

adguard:
  url: "http://127.0.0.1:3000"
  username: "admin"
  password: "..."                         # .credentials.enc'den

vpn:
  tunnels:
    - name: "nl-amsterdam"
      endpoint: "1.2.3.4:51820"
      private_key: "..."                  # .credentials.enc
      public_key: "..."
      allowed_ips: "0.0.0.0/0"
      dns: "10.0.0.1"
      table: 100
      fwmark: 100
  device_assignments:
    "aa:bb:cc:dd:ee:ff": "nl-amsterdam"

nas:
  shares:
    - name: "media"
      path: "/mnt/raid/media"
      guest_ok: true
      read_only: true
    - name: "backups"
      path: "/mnt/raid/backups"
      guest_ok: false
      valid_users: ["admin"]
  m3u_sources:
    - url: "http://example.com/playlist.m3u"
      download_path: "/mnt/raid/media/iptv"
      schedule: "0 4 * * *"              # Her gün 04:00

storage:
  raid:
    device: "/dev/md0"
    level: 1
    members: ["/dev/sda1", "/dev/sdb1"]
  smart_check_interval: 3600
```

---

## API Endpoint Inventory

### Auth
| Method | Path              | Açıklama                  |
|--------|-------------------|---------------------------|
| POST   | /api/auth/login   | Oturum aç (bcrypt + JWT)  |
| POST   | /api/auth/logout  | Oturum kapat              |
| GET    | /api/auth/me      | Mevcut kullanıcı bilgisi  |

### Dashboard
| Method | Path                  | Açıklama                         |
|--------|-----------------------|----------------------------------|
| GET    | /api/dashboard        | Özet istatistikler               |
| WS     | /api/ws/stats         | Real-time sistem metrikleri      |

### PPPoE
| Method | Path                  | Açıklama                         |
|--------|-----------------------|----------------------------------|
| GET    | /api/pppoe/status     | Bağlantı durumu + uptime         |
| POST   | /api/pppoe/connect    | PPPoE bağlantısını başlat        |
| POST   | /api/pppoe/disconnect | PPPoE bağlantısını kes           |
| PUT    | /api/pppoe/config     | PPPoE ayarlarını güncelle        |

### Firewall
| Method | Path                       | Açıklama                    |
|--------|----------------------------|-----------------------------|
| GET    | /api/firewall/rules        | Aktif nftables kuralları    |
| PUT    | /api/firewall/rules        | Kuralları güncelle          |
| GET    | /api/firewall/port-forwards| Port yönlendirme listesi    |
| POST   | /api/firewall/port-forwards| Yeni port yönlendirme ekle  |
| DELETE | /api/firewall/port-forwards/{id} | Port yönlendirme sil  |

### AdGuard Home
| Method | Path                    | Açıklama                      |
|--------|-------------------------|-------------------------------|
| GET    | /api/adguard/status     | AGH genel durum               |
| GET    | /api/adguard/stats      | DNS istatistikleri            |
| GET    | /api/adguard/leases     | DHCP lease listesi            |
| POST   | /api/adguard/lease      | Statik lease ekle             |
| DELETE | /api/adguard/lease/{mac}| Lease sil                     |

### QoS
| Method | Path                | Açıklama                        |
|--------|---------------------|---------------------------------|
| GET    | /api/qos/status     | Aktif QoS profili + istatistik  |
| PUT    | /api/qos/profile    | Profil değiştir (CAKE/fq_codel) |
| PUT    | /api/qos/limits     | Bant genişliği limitleri        |
| PUT    | /api/qos/congestion | Congestion control (BBR/CUBIC)  |

### VPN
| Method | Path                          | Açıklama                    |
|--------|-------------------------------|-----------------------------|
| GET    | /api/vpn/tunnels              | WireGuard tünel listesi     |
| POST   | /api/vpn/tunnels              | Yeni tünel ekle             |
| DELETE | /api/vpn/tunnels/{name}       | Tünel sil                   |
| GET    | /api/vpn/assignments          | Cihaz-tünel atamaları       |
| PUT    | /api/vpn/assignments          | Cihaz atamasını güncelle    |
| GET    | /api/vpn/tunnels/{name}/peers | Peer listesi + transfer     |

### NAS
| Method | Path                    | Açıklama                      |
|--------|-------------------------|-------------------------------|
| GET    | /api/nas/shares         | Samba paylaşım listesi        |
| POST   | /api/nas/shares         | Yeni paylaşım ekle            |
| PUT    | /api/nas/shares/{name}  | Paylaşım güncelle             |
| DELETE | /api/nas/shares/{name}  | Paylaşım sil                  |
| POST   | /api/nas/m3u/sync       | M3U dosyalarını indir + parse |
| GET    | /api/nas/m3u/status     | M3U senkronizasyon durumu     |

### Storage
| Method | Path                   | Açıklama                       |
|--------|------------------------|--------------------------------|
| GET    | /api/storage/raid      | RAID durumu (mdadm)            |
| GET    | /api/storage/smart     | Disk sağlık bilgileri          |
| GET    | /api/storage/usage     | Disk kullanım istatistikleri   |

### System
| Method | Path                   | Açıklama                       |
|--------|------------------------|--------------------------------|
| GET    | /api/system/info       | Hostname, uptime, kernel       |
| POST   | /api/system/reboot     | Sistemi yeniden başlat         |
| GET    | /api/system/logs       | journalctl çıktısı (paginated)|
| POST   | /api/backup/export     | Config dışa aktar (.tar.gz)   |
| POST   | /api/backup/import     | Config içe aktar               |

---

## Implementation Phases

### Phase 1: Proje İskeleti + IPC Altyapısı (3 gün)
**Hedef:** Temel proje yapısını, privilege-separated agent/web mimarisini ve IPC mekanizmasını kurmak.

Oluşturulacak dosyalar:
- `pyproject.toml` — proje metadata, bağımlılıklar (fastapi, uvicorn, pyyaml, cryptography, jinja2, httpx, bcrypt, pyjwt)
- `home_router/__init__.py`, `home_router/main.py` — FastAPI app factory
- `home_router/agent/server.py` — Root agent UDS listener
- `home_router/agent/client.py` — Agent IPC client
- `home_router/agent/operations.py` — Op whitelist tanımları
- `home_router/config/manager.py` — YAML config loader + Fernet encryption
- `home_router/config/schema.py` — Pydantic modeller
- `home_router/utils/atomic.py` — AtomicChange context manager
- `home_router/utils/process.py` — Güvenli subprocess wrapper
- `systemd/home-router-agent.service`
- `systemd/home-router-web.service`
- `systemd/home-router.target`
- `config/router.yaml` — Varsayılan yapılandırma
- `scripts/install.sh` — Temel kurulum

Adımlar:
1. pyproject.toml + requirements.txt oluştur
2. Config manager: YAML load/save, Fernet encrypt/decrypt, atomic write (tmp→fsync→rename)
3. Agent server: asyncio UDS listener, JSON-based protocol, op whitelist dispatcher
4. Agent client: async context manager, request/response, timeout
5. FastAPI app: lifespan event'te agent client bağlantısı
6. systemd unit dosyaları: iki servis + target
7. install.sh: venv oluştur, bağımlılık kur, systemd enable

Manuel doğrulama:
- `systemctl start home-router.target` ile her iki servis de ayakta mı
- Agent socket'e test mesaj gönder, yanıt al
- `curl http://10.0.0.1:8443/api/health` yanıt veriyor mu

### Phase 2: Auth + Base API (2 gün)
**Hedef:** Oturum yönetimi, CSRF koruması ve temel API altyapısını kurmak.

Oluşturulacak dosyalar:
- `home_router/api/deps.py` — Auth dependency (JWT verify)
- `home_router/api/middleware.py` — CSRF, rate limit, LAN-only binding
- `home_router/api/routes/auth.py` — Login/logout/me
- `home_router/utils/crypto.py` — bcrypt hash, JWT sign/verify
- `frontend/index.html` — Login sayfası shell
- `frontend/js/api.js` — Fetch wrapper + token yönetimi
- `frontend/css/variables.css` + `frontend/css/reset.css`

Adımlar:
1. bcrypt ile admin password hash, JWT token oluşturma/doğrulama
2. Login endpoint: username/password → JWT token (httpOnly cookie)
3. Auth dependency: her korumalı route'ta JWT doğrulama
4. Rate limiting middleware (sliding window, IP tabanlı)
5. CSRF: double-submit cookie pattern
6. LAN-only binding: web sadece LAN interface IP'sinde dinler
7. Frontend login sayfası + API wrapper

Manuel doğrulama:
- Yanlış şifre ile 401 dönüyor mu
- JWT token ile korumalı endpoint'e erişim sağlanıyor mu
- WAN interface'den erişim engellenmiş mi

### Phase 3: PPPoE WAN Bağlantısı (3 gün)
**Hedef:** PPPoE üzerinden internete bağlanmak, auto-reconnect, bağlantı durumu izleme.

Oluşturulacak dosyalar:
- `home_router/services/pppoe_service.py`
- `home_router/templates/pppoe/peer.j2`
- `home_router/templates/pppoe/options.j2`
- `home_router/api/routes/pppoe.py`
- `home_router/api/routes/network.py`
- `frontend/js/pages/network.js`

Adımlar:
1. Jinja2 şablonları: `/etc/ppp/peers/wan` ve pppd options dosyası
2. PPPoE service: connect (pppd başlat), disconnect (pppd kill), status (pppd pid + interface durumu)
3. Credentials .credentials.enc'den Fernet ile çözülecek
4. Auto-reconnect: pppd `persist` + `holdoff` + service watchdog
5. Agent operations: `pppoe.connect`, `pppoe.disconnect`, `pppoe.status`
6. WAN interface IP, gateway, uptime bilgisi
7. MTU 1492 + MSS clamping hazırlığı (Phase 4'te uygulanacak)

Manuel doğrulama:
- `ppp0` interface'i ayağa kalkıyor mu
- İnternet erişimi var mı (`ping 8.8.8.8`)
- Bağlantı koptuğunda auto-reconnect çalışıyor mu
- Web UI'dan bağlantı durumu görünüyor mu

### Phase 4: nftables Firewall + NAT (5 gün)
**Hedef:** Zone-based firewall, NAT masquerade, MSS clamping, port forwarding.

Oluşturulacak dosyalar:
- `home_router/services/firewall_service.py`
- `home_router/templates/nftables/main.nft.j2`
- `home_router/agent/watchdog.py` — 30s rollback watchdog
- `home_router/api/routes/firewall.py`
- `config/firewall.yaml`
- `frontend/js/pages/firewall.js`

Adımlar:
1. nftables Jinja2 şablonu:
   - `table inet filter` — input/forward/output chains
   - `table ip nat` — prerouting (DNAT) + postrouting (masquerade)
   - MSS clamping: `tcp flags syn tcp option maxseg size set rt mtu` (PPPoE 1492 MTU)
   - Connection tracking: `ct state established,related accept`
   - WAN input: default drop, sadece established/related + ICMP
   - LAN→WAN forward: accept (NAT masquerade ile)
   - Rate limiting: `limit rate` ile SSH/HTTP brute force koruması
2. AtomicChange: `nft -c -f` (validate) → snapshot → apply → rollback on failure
3. Watchdog: apply sonrası 30s içinde web'den `confirm` gelmezse otomatik rollback
4. Port forwarding: DNAT + forward kuralı ekleme/silme
5. sysctl ayarları: `net.ipv4.ip_forward=1`, `net.ipv6.conf.all.forwarding=0`
6. Agent operations: `firewall.apply`, `firewall.confirm`, `firewall.rollback`

Manuel doğrulama:
- LAN'dan internete çıkılabiliyor mu (NAT çalışıyor mu)
- WAN'dan LAN'a erişim engellenmiş mi
- Port forwarding test: dış IP:port → LAN cihaz
- `nft list ruleset` beklenen kuralları gösteriyor mu
- Rollback çalışıyor mu (web'den confirm göndermeden bekleme)

### Phase 5: AdGuard Home Entegrasyonu (2 gün)
**Hedef:** AGH REST API üzerinden DHCP lease yönetimi, DNS istatistikleri, engelleme bilgileri.

Oluşturulacak dosyalar:
- `home_router/services/adguard_service.py`
- `home_router/api/routes/adguard.py`
- `frontend/js/pages/dns.js`

Adımlar:
1. httpx async client ile AGH REST API wrapper
2. DHCP leases: liste, statik lease ekle/sil
3. DNS stats: top clients, top domains, blocked queries
4. Genel durum: AGH version, çalışıyor mu, filtering enabled
5. Cihaz listesi: MAC + IP + hostname (diğer modüller tarafından kullanılacak)
6. Frontend: DNS istatistikleri sayfası, DHCP lease tablosu

Manuel doğrulama:
- AGH API'den veri geliyor mu
- Statik lease eklenip silinebiliyor mu
- DNS istatistikleri dashboard'da görünüyor mu

### Phase 6: Web Dashboard + WebSocket (4 gün)
**Hedef:** Ana dashboard, real-time sistem metrikleri, responsive layout, tema desteği.

Oluşturulacak dosyalar:
- `home_router/services/monitor_service.py`
- `home_router/api/routes/dashboard.py`
- `home_router/api/routes/ws.py`
- `frontend/index.html` (tam SPA shell)
- `frontend/css/layout.css`, `frontend/css/components.css`, `frontend/css/pages.css`
- `frontend/js/app.js` — Client-side router
- `frontend/js/ws.js` — WebSocket client
- `frontend/js/components/sidebar.js`, `chart.js`, `toast.js`, `modal.js`
- `frontend/js/pages/dashboard.js`
- `frontend/js/pages/settings.js`
- `frontend/assets/icons/*.svg`

Adımlar:
1. Monitor service: asyncio background task — CPU, RAM, temperature, network throughput (psutil + /proc/net/dev)
2. WebSocket endpoint: real-time broadcast (1 saniye interval)
3. Frontend SPA: hash-based routing (#/dashboard, #/network, #/firewall, ...)
4. Dashboard sayfası: uptime, WAN IP, throughput grafikleri (Canvas API), aktif cihaz sayısı
5. Sidebar navigasyon
6. Dark/light tema: CSS custom properties + `prefers-color-scheme`
7. Responsive layout: CSS Grid + mobile-first
8. Toast notification sistemi
9. Settings sayfası: hostname, timezone, password değiştir

Manuel doğrulama:
- Dashboard'da real-time CPU/RAM/bandwidth grafikleri güncelleniyor mu
- Mobil cihazdan LAN üzerinden erişilebilir mi
- Tema değişimi çalışıyor mu
- Sidebar navigasyonu tüm sayfalar arası geçiş yapıyor mu

### Phase 7: SQM/QoS — Bufferbloat Çözümü (3 gün)
**Hedef:** CAKE qdisc, per-device bandwidth limitleri, BBR/CUBIC congestion control.

Oluşturulacak dosyalar:
- `home_router/services/qos_service.py`
- `home_router/api/routes/qos.py`
- `config/qos.yaml`
- `frontend/js/pages/qos.js`

Adımlar:
1. CAKE qdisc uygulama:
   - Egress: `tc qdisc add dev ppp0 root cake bandwidth {upload}kbit`
   - Ingress: IFB (Intermediate Functional Block) device oluştur
   - `tc qdisc add dev ifb0 root cake bandwidth {download}kbit wash ingress`
2. Per-device bant genişliği limitleri: `tc filter` + `tc class` ile HTB
3. Congestion control: `sysctl net.ipv4.tcp_congestion_control={bbr|cubic}`
4. BBR ek: `sysctl net.core.default_qdisc=fq` (BBR için gerekli)
5. QoS profilleri: cake (varsayılan), fq_codel, none
6. Agent operations: `qos.apply`, `qos.clear`
7. Frontend: profil seçimi, bant genişliği ayarı, per-device limitleri

Manuel doğrulama:
- `tc -s qdisc show dev ppp0` CAKE gösteriyor mu
- Bufferbloat testi: `flent rrul` veya DSLReports Speed Test
- Online oyun sırasında download yapılırken latency artmıyor mu
- Per-device limit çalışıyor mu

### Phase 8: WireGuard VPN + Policy Routing (5 gün)
**Hedef:** WireGuard tünelleri, per-device VPN routing, drag-and-drop UI.

Oluşturulacak dosyalar:
- `home_router/services/vpn_service.py`
- `home_router/templates/wireguard/wg.conf.j2`
- `home_router/api/routes/vpn.py`
- `config/vpn.yaml`
- `frontend/js/components/drag-drop.js`
- `frontend/js/pages/vpn.js`

Adımlar:
1. WireGuard config şablonu: private/public key, endpoint, allowed IPs, DNS
2. Tünel yönetimi: `wg-quick up/down wgN`, tünel ekleme/silme
3. Policy routing:
   - Her tünel için: `ip route add default dev wgN table {table_id}`
   - Cihaz ataması: nftables'da kaynak MAC/IP'ye fwmark
   - `ip rule add fwmark {mark} lookup {table_id}`
   - `ct mark` ile reply paketlerde fwmark korunması
4. nftables şablonu güncelleme: VPN fwmark chain ekleme
5. Keypair oluşturma: `wg genkey | tee private | wg pubkey > public`
6. Frontend drag-and-drop:
   - Sol panel: cihaz listesi (AGH DHCP'den)
   - Sağ panel: aktif VPN tünelleri (drop zone)
   - Drag: cihazı tünele sürükle → API çağrısı → policy route ekle
7. Kill switch: VPN düşerse o cihazın trafiğini engelle (opsiyonel, config'den)

Manuel doğrulama:
- WireGuard tünel bağlantısı kurulabiliyor mu (`wg show`)
- Atanmış cihaz VPN üzerinden çıkıyor mu (whatismyip kontrolü)
- Atanmamış cihaz normal PPPoE'den çıkmaya devam ediyor mu
- Drag-and-drop ile cihaz ataması anlık çalışıyor mu
- Tünel düştüğünde kill switch çalışıyor mu

### Phase 9: Samba NAS + M3U Parser (3 gün)
**Hedef:** Samba paylaşımları, M3U dosya indirme/parse, Kodi uyumlu medya yapısı.

Oluşturulacak dosyalar:
- `home_router/services/nas_service.py`
- `home_router/templates/samba/smb.conf.j2`
- `home_router/api/routes/nas.py`
- `config/nas.yaml`
- `frontend/js/pages/nas.js`

Adımlar:
1. Samba config şablonu: global ayarlar + per-share tanımlar
2. Paylaşım CRUD: oluştur, güncelle, sil → `smb.conf` regenerate → `smbcontrol reload-config`
3. M3U parser:
   - M3U/M3U8 dosyası indir (httpx)
   - `#EXTINF` satırlarını parse et: grup, başlık, URL
   - İçerikleri gruplara göre klasörlere indir
   - Kodi-friendly .strm dosyaları oluştur
4. Zamanlı M3U senkronizasyonu (cron veya asyncio scheduled task)
5. Agent operations: `samba.reload`
6. Frontend: paylaşım listesi, M3U kaynak yönetimi, senkronizasyon durumu

Manuel doğrulama:
- Windows/macOS/Linux'tan Samba paylaşımına erişilebiliyor mu
- M3U parse doğru çalışıyor mu (dosya yapısı kontrol)
- Kodi'den medya oynatılabiliyor mu
- Yeni paylaşım eklendiğinde `smbclient -L` listede görünüyor mu

### Phase 10: Storage + Backup + Hardening (4 gün)
**Hedef:** RAID durumu izleme, disk sağlığı, config yedekleme, güvenlik sertleştirme.

Oluşturulacak dosyalar:
- `home_router/services/storage_service.py`
- `home_router/services/backup_service.py`
- `home_router/api/routes/storage.py`
- `frontend/js/pages/storage.js`
- `scripts/factory-reset.sh`
- `scripts/backup.sh`

Adımlar:
1. RAID izleme: `mdadm --detail /dev/md0` parse, degraded uyarısı
2. SMART: `smartctl -a /dev/sdX` → disk sağlık skoru, sıcaklık, hata sayısı
3. Disk kullanımı: `df`, `du` ile paylaşım bazında kullanım
4. Config backup:
   - Export: `config/` dizinini + AGH config'i tar.gz olarak paketle
   - Import: tar.gz'den çöz, doğrula, uygula, servisleri restart
   - Zamanlanmış backup: günlük RAID'e, haftalık harici
5. Factory reset: varsayılan config'e dön, tüm servisleri restart
6. Güvenlik sertleştirme:
   - `fail2ban` veya kendi rate limiter (Phase 2'deki middleware)
   - SSH: sadece key auth, LAN only
   - sysctl hardening: rp_filter, tcp_syncookies, icmp_ignore_bogus
   - systemd: ProtectSystem, PrivateTmp, NoNewPrivileges
   - Otomatik güvenlik güncellemeleri: `unattended-upgrades`
7. HDD spin-up stagger: `hdparm -S` ile PicoPSU koruma

Manuel doğrulama:
- RAID durumu dashboard'da doğru görünüyor mu
- SMART verileri okunuyor mu
- Config export → factory reset → config import çalışıyor mu
- fail2ban / rate limit brute force'u engelliyor mu

---

## Veri Akış Diyagramları

### PPPoE Bağlantı Akışı
```
Web UI → POST /api/pppoe/connect
  → auth middleware (JWT doğrula)
  → pppoe_service.connect()
    → config'den credentials çöz (Fernet)
    → Jinja2: /etc/ppp/peers/wan oluştur
    → agent_client.call("pppoe.connect")
      → Agent: subprocess("pppd call wan")
      → ppp0 interface ayağa kalkar
      → Agent: return {status: "connected", ip: "..."}
    → pppoe_service: firewall_service.apply() tetikle
      → NAT masquerade + MSS clamping aktif
```

### VPN Cihaz Atama Akışı
```
Web UI → PUT /api/vpn/assignments {mac: "aa:bb:...", tunnel: "nl-amsterdam"}
  → vpn_service.assign_device(mac, tunnel)
    → vpn.yaml güncelle (atomic write)
    → nftables fwmark kuralı oluştur:
        meta mark set {fwmark} ip saddr {device_ip}
    → agent_client.call("firewall.apply", nft_rules)
    → agent_client.call("routing.add_rule", {fwmark, table})
      → ip rule add fwmark 100 lookup 100
    → return {status: "assigned"}
  → WebSocket: tüm client'lara "assignment_changed" event
```

### Atomic Firewall Change Akışı
```
firewall_service.apply(new_rules)
  → AtomicChange(service="firewall"):
    → snapshot(): nft list ruleset > /tmp/nft-backup-{ts}
    → validate(): nft -c -f /tmp/new-rules.nft (dry run)
    → apply(): agent_client.call("firewall.apply")
    → watchdog.start(30s)
      → 30s içinde confirm() gelirse: snapshot sil, watchdog iptal
      → 30s içinde confirm() gelmezse: rollback → eski ruleset restore
```

---

## Bağımlılıklar (pyproject.toml)

```
fastapi >= 0.115
uvicorn[standard] >= 0.32
pyyaml >= 6.0
cryptography >= 43.0      # Fernet encryption
jinja2 >= 3.1
httpx >= 0.28              # AGH API client, async
bcrypt >= 4.2
pyjwt >= 2.9
psutil >= 6.0              # Sistem metrikleri
websockets >= 13.0         # FastAPI WebSocket desteği
pydantic >= 2.9            # Config validation
```

## Sistem Gereksinimleri (install.sh)

```
apt install -y \
  python3.12 python3.12-venv \
  ppp pppoe \
  nftables \
  wireguard-tools \
  samba samba-common-bin \
  smartmontools mdadm \
  iproute2 \
  adguardhome
```

---

## Risks and Trade-offs

| Risk                                    | Mitigation                                                              |
|-----------------------------------------|-------------------------------------------------------------------------|
| PMTU black-holing (PPPoE MTU 1492)      | Phase 4'te MSS clamping zorunlu                                        |
| NIC isimlendirme değişimi (reboot)      | udev rules by MAC address (`setup-interfaces.sh`)                      |
| VPN policy route'lar reboot'ta kaybolur | Agent startup'ta `vpn.yaml`'dan restore                                |
| Firewall kuralı hatalı → ağ kilitlenir | AtomicChange + 30s watchdog rollback                                   |
| PicoPSU 180W, 6 disk ile surge riski   | HDD spin-up stagger (`hdparm -S`)                                      |
| Web UI XSS → firewall manipülasyonu    | CSP header, input sanitization, agent op whitelist                     |
| PPPoE credential sızıntısı             | Fernet encryption at rest, memory-only decrypt                         |
| AGH API erişilemez → DHCP bilgisi yok  | Cache layer + health check, degraded mode UI uyarısı                   |
| Single point of failure (tek cihaz)    | Config backup + factory reset + RAID-1 depolama                        |
| Ubuntu unattended-upgrade bozma riski  | Güvenlik güncellemeleri sadece, kernel pin                              |

## Tahmini Toplam Süre

| Phase | Gün | Kümülatif |
|-------|-----|-----------|
| 1     | 3   | 3         |
| 2     | 2   | 5         |
| 3     | 3   | 8         |
| 4     | 5   | 13        |
| 5     | 2   | 15        |
| 6     | 4   | 19        |
| 7     | 3   | 22        |
| 8     | 5   | 27        |
| 9     | 3   | 30        |
| 10    | 4   | 34        |

**Toplam: ~34 geliştirme günü** (tek geliştirici, her gün 4-6 saat efektif çalışma varsayımı)
