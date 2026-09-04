import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { Component } from '@angular/core';
import { By } from '@angular/platform-browser';
import { RouterTestingModule } from '@angular/router/testing';
import { TranslateModule, TranslateService } from '@ngx-translate/core';

import { UiMenuComponent } from './menu.component';
import { UiMenuItem } from './menu-item.model';

@Component({
    selector: 'ui-test-host',
    template: `
        <button #trigger (click)="menu.toggle($event)">open</button>
        <ui-menu #menu [model]="items"></ui-menu>
    `,
    standalone: false
})
class TestHostComponent {
    public readonly items: UiMenuItem[] = [
        { labelKey: 'PROJECT.CHATS', command: () => {} },
        { label: 'Static label', command: () => {} }
    ];
}

describe('UiMenuComponent default row template', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [UiMenuComponent, TestHostComponent],
            imports: [RouterTestingModule, TranslateModule.forRoot()]
        }).compileComponents();
    });

    it('resolves labelKey through the translate pipe and falls back to label', () => {
        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            PROJECT: { CHATS: 'Project chats' }
        });
        translate.use('en');

        const fixture = TestBed.createComponent(TestHostComponent);
        fixture.detectChanges();

        // Open the popup so the menu template is rendered in the DOM.
        fixture.debugElement.query(By.css('button')).nativeElement.click();
        fixture.detectChanges();

        const labels = document.querySelectorAll('.ui-menu-item-label');
        expect(labels.length).toBe(2);
        expect(labels[0].textContent.trim()).toBe('Project chats');
        expect(labels[1].textContent.trim()).toBe('Static label');
    });
});
