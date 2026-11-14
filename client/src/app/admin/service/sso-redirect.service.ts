import { DOCUMENT } from '@angular/common';
import { Injectable, inject } from '@angular/core';

@Injectable({ providedIn: 'root' })
export class SsoRedirect {
    private readonly document = inject(DOCUMENT);

    public go(url: string): void {
        this.document.location.assign(url);
    }
}
