import { Injectable, signal, TemplateRef } from '@angular/core';

@Injectable()
export class IssueToolbarService {
    public readonly toolbarTemplate = signal<TemplateRef<any> | null>(null);

    public register(template: TemplateRef<any>): void {
        this.toolbarTemplate.set(template);
    }

    public clear(): void {
        this.toolbarTemplate.set(null);
    }
}
