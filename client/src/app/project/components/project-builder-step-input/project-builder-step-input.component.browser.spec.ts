import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ProjectBuilderStepInputComponent } from './project-builder-step-input.component';
import { TranslateModule } from '@ngx-translate/core';
import { FormsModule, NG_VALUE_ACCESSOR, ControlValueAccessor } from '@angular/forms';
import { UiModule } from '../../../ui/ui.module';
import { TablerIconComponent, provideTablerIcons, IconBolt } from '@tabler/icons-angular';
import { ChangeDetectionStrategy, Component, Input, forwardRef } from '@angular/core';

// Minimal CVA stub for the severity dropdown so ngModel binds without pulling in
// the real component (which has its own dependency tree).
@Component({
    selector: 'app-severity-dropdown',
    template: '',
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => SeverityDropdownStub),
            multi: true
        }
    ]
})
class SeverityDropdownStub implements ControlValueAccessor {
    @Input() public severities: unknown;
    @Input() public inputId: unknown;
    public writeValue(): void {}
    public registerOnChange(): void {}
    public registerOnTouched(): void {}
}

// Minimal CVA stub for the state dropdown so ngModel binds without pulling in
// the real component (which has its own dependency tree).
@Component({
    selector: 'app-state-dropdown',
    template: '',
    standalone: false,
    providers: [
        {
            provide: NG_VALUE_ACCESSOR,
            useExisting: forwardRef(() => StateDropdownStub),
            multi: true
        }
    ]
})
class StateDropdownStub implements ControlValueAccessor {
    @Input() public states: unknown;
    @Input() public inputId: unknown;
    public writeValue(): void {}
    public registerOnChange(): void {}
    public registerOnTouched(): void {}
}

describe('ProjectBuilderStepInputComponent', () => {
    let fixture: ComponentFixture<ProjectBuilderStepInputComponent>;
    let component: ProjectBuilderStepInputComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [
                ProjectBuilderStepInputComponent,
                SeverityDropdownStub,
                StateDropdownStub
            ],
            imports: [TranslateModule.forRoot(), FormsModule, UiModule, TablerIconComponent],
            providers: [provideTablerIcons({ IconBolt })]
        })
            .overrideComponent(ProjectBuilderStepInputComponent, {
                set: { changeDetection: ChangeDetectionStrategy.Default }
            })
            .compileComponents();

        fixture = TestBed.createComponent(ProjectBuilderStepInputComponent);
        component = fixture.componentInstance;
    });

    it('emits generate event when onGenerate is called', () => {
        fixture.componentRef.setInput('description', 'valid description text');
        fixture.componentRef.setInput('isGenerating', false);
        fixture.componentRef.setInput('isGenerateDisabled', false);
        fixture.componentRef.setInput('rateLimitCountdown', 0);
        fixture.componentRef.setInput('states', []);
        fixture.componentRef.setInput('severities', []);

        let emittedCount = 0;
        component.generate.subscribe(() => emittedCount++);
        component.onGenerate();
        expect(emittedCount).toBe(1);
    });

    it('does not emit generate when isGenerateDisabled is true', () => {
        const emitted: void[] = [];
        component.generate.subscribe(() => emitted.push());
        fixture.componentRef.setInput('isGenerating', false);
        fixture.componentRef.setInput('isGenerateDisabled', true);
        fixture.componentRef.setInput('rateLimitCountdown', 0);
        fixture.componentRef.setInput('states', []);
        fixture.componentRef.setInput('severities', []);
        fixture.componentRef.setInput('description', 'short');
        fixture.detectChanges();

        const btn = fixture.nativeElement.querySelector('ui-button button') as HTMLButtonElement;
        expect(btn.disabled).toBe(true);
    });
});
