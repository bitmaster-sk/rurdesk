import { Injectable } from '@angular/core';
import { Tag } from '../model/tag.model';

@Injectable({
    providedIn: 'root'
})
export class TagColorService {
    public readonly palette: readonly string[] = [
        '#dc2626',
        '#ea580c',
        '#d97706',
        '#16a34a',
        '#0d9488',
        '#0284c7',
        '#2563eb',
        '#9333ea',
        '#db2777',
        '#75665c'
    ];

    public nextColor(tags: Pick<Tag, 'color'>[]): string {
        const used = new Set(tags.map(tag => tag.color.toLowerCase()));
        return (
            this.palette.find(color => !used.has(color)) ??
            this.palette[tags.length % this.palette.length]
        );
    }
}
