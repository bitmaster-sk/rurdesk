import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { UiButtonStub } from 'src/testing/stubs';
import { SplitInputStepComponent } from './split-input-step.component';

describe('SplitInputStepComponent', () => {
    let component: SplitInputStepComponent;
    let fixture: ComponentFixture<SplitInputStepComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [SplitInputStepComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot(), UiButtonStub]
        }).compileComponents();

        fixture = TestBed.createComponent(SplitInputStepComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('isLoading', false);
        fixture.componentRef.setInput('issueTitle', 'My Issue');
        fixture.detectChanges();
    });

    it('should create', () => {
        expect(component).toBeTruthy();
    });

    it('emits split with hint value when onSplit() is called', () => {
        let emitted: string | undefined;
        component.split.subscribe((v: string) => (emitted = v));
        component.hintControl.setValue('split by feature');
        component.onSplit();
        expect(emitted).toBe('split by feature');
    });

    it('emits split with empty string when hint is blank', () => {
        let emitted: string | undefined;
        component.split.subscribe((v: string) => (emitted = v));
        component.onSplit();
        expect(emitted).toBe('');
    });

    it('emits cancelled when onCancel() is called', () => {
        let emitted = false;
        component.cancelled.subscribe(() => (emitted = true));
        component.onCancel();
        expect(emitted).toBe(true);
    });
});
