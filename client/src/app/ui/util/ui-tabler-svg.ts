import { TablerIcon } from '@tabler/icons-angular';

const SVG_NS = 'http://www.w3.org/2000/svg';

export abstract class UiTablerSvg {
    public static create(icon: TablerIcon, size: number, className = ''): SVGSVGElement {
        const svg = document.createElementNS(SVG_NS, 'svg');
        const isFilled = icon.type === 'filled';
        svg.setAttribute('viewBox', '0 0 24 24');
        svg.setAttribute('width', String(size));
        svg.setAttribute('height', String(size));
        svg.setAttribute('fill', isFilled ? 'currentColor' : 'none');
        svg.setAttribute('stroke', isFilled ? 'none' : 'currentColor');
        svg.setAttribute('stroke-width', '2');
        svg.setAttribute('stroke-linecap', 'round');
        svg.setAttribute('stroke-linejoin', 'round');
        svg.setAttribute('aria-hidden', 'true');
        svg.dataset['icon'] = icon.name;
        if (className) {
            svg.setAttribute('class', className);
        }
        for (const [elementName, attrs] of icon.nodes) {
            const child = document.createElementNS(SVG_NS, elementName);
            for (const [name, value] of Object.entries(attrs)) {
                if (name !== 'key') {
                    child.setAttribute(name, value);
                }
            }
            svg.append(child);
        }
        return svg;
    }
}
