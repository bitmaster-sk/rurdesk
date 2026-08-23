import { Injectable, inject } from '@angular/core';
import { Resolve } from '@angular/router';
import { IssueTypeStore } from './store/issue-type.store';

@Injectable({ providedIn: 'root' })
export class IssueTypeResolver implements Resolve<void> {
    private issueTypeStore = inject(IssueTypeStore);

    public resolve(): void {
        this.issueTypeStore.load();
    }
}
