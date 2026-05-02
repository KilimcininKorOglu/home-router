(function() {
    window.BandwidthChart = function(canvasId, maxPoints) {
        var canvas = document.getElementById(canvasId);
        if (!canvas) return null;
        var ctx = canvas.getContext('2d');
        var points = { rx: [], tx: [] };
        maxPoints = maxPoints || 60;

        function draw() {
            var w = canvas.width = canvas.offsetWidth;
            var h = canvas.height = canvas.offsetHeight;
            var pad = 4;

            ctx.clearRect(0, 0, w, h);

            var allValues = points.rx.concat(points.tx);
            var maxVal = Math.max.apply(null, allValues.length ? allValues : [1]);
            if (maxVal === 0) maxVal = 1;

            drawLine(points.rx, w, h, pad, maxVal, getComputedStyle(document.documentElement).getPropertyValue('--accent-blue').trim() || '#1D9BF0');
            drawLine(points.tx, w, h, pad, maxVal, getComputedStyle(document.documentElement).getPropertyValue('--accent-green').trim() || '#00BA7C');
        }

        function drawLine(data, w, h, pad, maxVal, color) {
            if (data.length < 2) return;
            ctx.beginPath();
            ctx.strokeStyle = color;
            ctx.lineWidth = 2;
            ctx.lineJoin = 'round';

            var step = (w - pad * 2) / (maxPoints - 1);
            var startIdx = Math.max(0, data.length - maxPoints);

            for (var i = startIdx; i < data.length; i++) {
                var x = pad + (i - startIdx) * step;
                var y = h - pad - ((data[i] / maxVal) * (h - pad * 2));
                if (i === startIdx) {
                    ctx.moveTo(x, y);
                } else {
                    ctx.lineTo(x, y);
                }
            }
            ctx.stroke();
        }

        return {
            addPoint: function(rx, tx) {
                points.rx.push(rx);
                points.tx.push(tx);
                if (points.rx.length > maxPoints) {
                    points.rx.shift();
                    points.tx.shift();
                }
                draw();
            },
            clear: function() {
                points = { rx: [], tx: [] };
                draw();
            }
        };
    };
})();
