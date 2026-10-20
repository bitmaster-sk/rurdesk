import { Component } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { FormControl, FormGroup, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { describe, expect, it } from 'vitest';
import { UiModule } from 'src/app/ui/ui.module';
import { AgentTurnLimitsFieldsComponent } from './agent-turn-limits-fields.component';

@Component({
    standalone: false,
    template: `
        <form [formGroup]="form"><app-agent-turn-limits-fields idPrefix="t" /></form>
    `
})
class HostComponent {
    public readonly form = new FormGroup({
        maxTurnsPlan: new FormControl<number | null>(120),
        maxTurnsImplement: new FormControl<number | null>(null)
    });
}

describe('AgentTurnLimitsFieldsComponent (browser)', () => {
    async function render() {
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule, TranslateModule.forRoot(), UiModule],
            declarations: [HostComponent, AgentTurnLimitsFieldsComponent]
        }).compileComponents();
        const fixture = TestBed.createComponent(HostComponent);
        fixture.detectChanges();
        return fixture;
    }

    it('shows the limits of the parent form', async () => {
        const fixture = await render();
        const root = fixture.nativeElement as HTMLElement;

        expect((root.querySelector('#t-max-turns-plan') as HTMLInputElement).value).toBe('120');
        expect((root.querySelector('#t-max-turns-implement') as HTMLInputElement).value).toBe('');
    });

    it('writes a typed limit into the parent form', async () => {
        const fixture = await render();
        const input = fixture.nativeElement.querySelector(
            '#t-max-turns-implement'
        ) as HTMLInputElement;

        input.value = '700';
        input.dispatchEvent(new Event('input'));

        expect(fixture.componentInstance.form.controls.maxTurnsImplement.value).toBe(700);
    });
});
