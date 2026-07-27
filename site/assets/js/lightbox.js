/**
 * Lightbox for docs + landing media.
 *
 * Every image/video emitted by mediaHtml() in tools/build.mjs carries
 * data-zoom; clicking (or Enter/Space on) one opens it full-size in an
 * overlay. No dependencies, works straight from file://.
 */
(function () {
    'use strict';

    var overlay = null;
    var stage = null;
    var caption = null;
    var lastFocused = null;

    function build() {
        overlay = document.createElement('div');
        overlay.className = 'lightbox';
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');
        overlay.innerHTML =
            '<button type="button" class="lightbox__close" aria-label="Close">&times;</button>' +
            '<figure class="lightbox__figure">' +
            '<div class="lightbox__stage"></div>' +
            '<figcaption class="lightbox__caption"></figcaption>' +
            '</figure>';

        stage = overlay.querySelector('.lightbox__stage');
        caption = overlay.querySelector('.lightbox__caption');

        overlay.addEventListener('click', function (event) {
            // Backdrop, close button and the caption strip all dismiss;
            // clicks on the media itself must not.
            if (!event.target.closest('.lightbox__stage') || event.target.closest('.lightbox__close')) {
                close();
            }
        });
        document.body.appendChild(overlay);
    }

    function open(source) {
        if (!overlay) {
            build();
        }
        lastFocused = source;
        stage.textContent = '';

        var isVideo = source.tagName === 'VIDEO';
        var media;
        if (isVideo) {
            media = document.createElement('video');
            media.src = source.currentSrc || source.src;
            media.autoplay = true;
            media.loop = true;
            media.muted = true;
            media.playsInline = true;
            media.controls = true;
        } else {
            media = document.createElement('img');
            media.src = source.currentSrc || source.src;
            media.alt = source.alt || '';
            // SVGs carrying only a viewBox have no intrinsic size and would
            // collapse in the shrink-to-fit stage — they need a definite width.
            if (/\.svg($|[?#])/i.test(media.src)) {
                media.className = 'is-vector';
            }
        }
        stage.appendChild(media);

        var figcaption = source.closest('figure') && source.closest('figure').querySelector('figcaption');
        caption.textContent = figcaption ? figcaption.textContent : source.alt || '';
        caption.hidden = !caption.textContent;

        document.body.classList.add('lightbox-open');
        overlay.classList.add('is-open');
        overlay.querySelector('.lightbox__close').focus();
    }

    function close() {
        if (!overlay || !overlay.classList.contains('is-open')) {
            return;
        }
        overlay.classList.remove('is-open');
        document.body.classList.remove('lightbox-open');
        stage.textContent = '';
        if (lastFocused) {
            lastFocused.focus();
            lastFocused = null;
        }
    }

    document.addEventListener('click', function (event) {
        var target = event.target.closest('[data-zoom]');
        if (target) {
            event.preventDefault();
            open(target);
        }
    });

    document.addEventListener('keydown', function (event) {
        if (event.key === 'Escape') {
            close();
            return;
        }
        if (event.key !== 'Enter' && event.key !== ' ') {
            return;
        }
        var target = event.target.closest && event.target.closest('[data-zoom]');
        if (target) {
            event.preventDefault();
            open(target);
        }
    });
})();
