import { Injectable, signal } from '@angular/core';

@Injectable()
export class LinkScope {
    public readonly idProject = signal<number | null>(null);
}
