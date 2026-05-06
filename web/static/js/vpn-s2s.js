// vpn-s2s.js — wizard glue. POSTs go through fetch() rather than
// htmx because the responses are JSON we need to splice into hidden
// form fields, not HTML fragments to swap.
(function () {
    'use strict';

    var ns = {};

    function el(id) { return document.getElementById(id); }

    function postForm(url, form) {
        var body = new URLSearchParams();
        new FormData(form).forEach(function (v, k) { body.append(k, v); });
        return fetch(url, {
            method: 'POST',
            headers: { 'Accept': 'application/json' },
            body: body,
            credentials: 'same-origin',
        }).then(function (res) {
            if (!res.ok) {
                return res.text().then(function (msg) { throw new Error(msg || ('HTTP ' + res.status)); });
            }
            return res.json();
        });
    }

    function fmtExpires(iso) {
        if (!iso) return '';
        try {
            var d = new Date(iso);
            return d.toLocaleString();
        } catch (e) {
            return iso;
        }
    }

    function bindIssue() {
        var f = el('s2s-issue-form');
        if (!f) return;
        f.addEventListener('submit', function (ev) {
            ev.preventDefault();
            postForm('/vpn/s2s/invite', f).then(function (data) {
                el('s2s-invite-token').value = data.token || '';
                el('s2s-finalize-peer').value = data.peerName || '';
                el('s2s-token-expires').textContent = data.expiresAt
                    ? ' (expires ' + fmtExpires(data.expiresAt) + ')'
                    : '';
                el('s2s-invite-result').style.display = 'block';
            }).catch(function (err) {
                alert(err.message);
            });
        });
    }

    function bindFinalize() {
        var f = el('s2s-finalize-form');
        if (!f) return;
        f.addEventListener('submit', function (ev) {
            ev.preventDefault();
            postForm('/vpn/s2s/finalize', f).then(function () {
                location.reload();
            }).catch(function (err) {
                alert(err.message);
            });
        });
    }

    function bindJoin() {
        var f = el('s2s-join-form');
        if (!f) return;
        f.addEventListener('submit', function (ev) {
            ev.preventDefault();
            postForm('/vpn/s2s/join', f).then(function (data) {
                el('s2s-ack-token').value = data.ackToken || '';
                el('s2s-join-result').style.display = 'block';
            }).catch(function (err) {
                alert(err.message);
            });
        });
    }

    ns.copyToken = function () {
        var ta = el('s2s-invite-token');
        ta.select();
        try { document.execCommand('copy'); } catch (e) { /* ignore */ }
        if (navigator.clipboard) {
            navigator.clipboard.writeText(ta.value).catch(function () { /* ignore */ });
        }
    };

    ns.copyAck = function () {
        var ta = el('s2s-ack-token');
        ta.select();
        try { document.execCommand('copy'); } catch (e) { /* ignore */ }
        if (navigator.clipboard) {
            navigator.clipboard.writeText(ta.value).catch(function () { /* ignore */ });
        }
    };

    ns.testReachability = function (name) {
        fetch('/vpn/s2s/' + encodeURIComponent(name) + '/reachability', {
            method: 'POST',
            credentials: 'same-origin',
        }).then(function (res) {
            if (res.status === 204) {
                alert('Reachable.');
            } else {
                return res.text().then(function (msg) { alert('Unreachable: ' + (msg || res.status)); });
            }
        }).catch(function (err) {
            alert(err.message);
        });
    };

    function init() {
        bindIssue();
        bindFinalize();
        bindJoin();
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }

    window.vpnS2S = ns;
})();
