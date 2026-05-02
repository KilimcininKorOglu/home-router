/* htmx-sortable placeholder - replace with actual Sortable.js + HTMX extension before deployment */
/* Download from: https://github.com/bigskysoftware/htmx-extensions/tree/main/src/sortable */
(function() {
    document.addEventListener('htmx:afterSettle', function() {
        document.querySelectorAll('[data-sortable]').forEach(function(el) {
            if (el._sortableInit) return;
            el._sortableInit = true;

            var items = el.children;
            Array.from(items).forEach(function(item) {
                item.draggable = true;
                item.style.cursor = 'grab';

                item.addEventListener('dragstart', function(e) {
                    e.dataTransfer.setData('text/plain', item.dataset.name || '');
                    item.style.opacity = '0.5';
                });

                item.addEventListener('dragend', function() {
                    item.style.opacity = '1';
                });

                item.addEventListener('dragover', function(e) {
                    e.preventDefault();
                });

                item.addEventListener('drop', function(e) {
                    e.preventDefault();
                    var draggedName = e.dataTransfer.getData('text/plain');
                    var names = Array.from(el.children).map(function(c) { return c.dataset.name; });
                    var fromIdx = names.indexOf(draggedName);
                    var toIdx = Array.from(el.children).indexOf(item);
                    if (fromIdx !== -1 && toIdx !== -1 && fromIdx !== toIdx) {
                        var moved = el.children[fromIdx];
                        if (fromIdx < toIdx) {
                            el.insertBefore(moved, item.nextSibling);
                        } else {
                            el.insertBefore(moved, item);
                        }
                        var newOrder = Array.from(el.children).map(function(c) { return c.dataset.name; });
                        fetch('/routing/reorder', {
                            method: 'POST',
                            headers: {'Content-Type': 'application/json'},
                            body: JSON.stringify(newOrder)
                        });
                    }
                });
            });
        });
    });
})();
