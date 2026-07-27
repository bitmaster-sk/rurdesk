import { describe, it, expect, beforeEach } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { TranslateModule } from '@ngx-translate/core';
import { UiModule } from '../../ui.module';
import { UiCommandHelpComponent } from './command-help.component';

describe('UiCommandHelpComponent', () => {
    beforeEach(() =>
        TestBed.configureTestingModule({ imports: [UiModule, TranslateModule.forRoot()] })
    );
    it('lists the core shortcuts', () => {
        const f = TestBed.createComponent(UiCommandHelpComponent);
        f.detectChanges();
        expect(f.nativeElement.textContent).toContain('COMMAND.HELP'); // translate echoes keys when unloaded
        expect(f.nativeElement.querySelectorAll('kbd').length).toBeGreaterThan(3);
    });
    it('renders a root data-help container (overlay owns Escape/backdrop close)', () => {
        const f = TestBed.createComponent(UiCommandHelpComponent);
        f.detectChanges();
        expect(f.nativeElement.querySelector('[data-help]')).not.toBeNull();
    });
});
