import { Injectable, signal } from '@angular/core';

@Injectable()
export class WikiLinkScope {
    public readonly idProject = signal<number | null>(null);
}
