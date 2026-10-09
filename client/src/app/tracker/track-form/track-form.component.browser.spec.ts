import { beforeEach, describe, expect, it, vi } from 'vitest';
import { Component, Directive, booleanAttribute, input } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { TrackFormComponent } from './track-form.component';

@Component({
    selector: 'ui-button',
    standalone: false,
    template: '<button [disabled]="disabled()" type="button"><ng-content /></button>'
})
class UiButtonStubComponent {
    public readonly label = input('');
    public readonly disabled = input(false);
    public readonly loading = input(false);
}

@Component({
    selector: 'tabler-icon',
    standalone: false,
    template: ''
})
class TablerIconStubComponent {
    public readonly icon = input('');
    public readonly size = input(16);
}

@Directive({
    selector: 'input[uiDatepicker]',
    standalone: false,
    host: {
        '[class.ui-input--invalid]': 'invalid()'
    }
})
class UiDatepickerStubDirective {
    public readonly invalid = input(false, { transform: booleanAttribute });
}

const TRACK: TrackForm = {
    idTrack: 5,
    idIssue: 10,
    idUser: 7,
    tracked: 3661,
    endAt: new Date('2026-10-09T10:00:00'),
    note: null
};

const NEW_TRACK: TrackForm = {
    ...TRACK,
    idTrack: null,
    tracked: 60,
    endAt: TRACK.endAt
};

describe('TrackFormComponent (browser)', () => {
    let trackerService: {
        insertTrack$: ReturnType<typeof vi.fn>;
        updateTrack$: ReturnType<typeof vi.fn>;
    };

    beforeEach(async () => {
        trackerService = {
            insertTrack$: vi.fn().mockReturnValue(of(undefined)),
            updateTrack$: vi.fn().mockReturnValue(of(undefined))
        };

        await TestBed.configureTestingModule({
            declarations: [
                TrackFormComponent,
                UiButtonStubComponent,
                TablerIconStubComponent,
                UiDatepickerStubDirective
            ],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()],
            providers: [{ provide: TrackerService, useValue: trackerService }]
        }).compileComponents();
    });

    it('re-renders the tracked input when the track input changes', () => {
        const fixture = TestBed.createComponent(TrackFormComponent);
        fixture.componentRef.setInput('track', TRACK);
        fixture.detectChanges();

        const trackedInput: HTMLInputElement = fixture.nativeElement.querySelector('input#tracked');
        expect(trackedInput.value).toBe(
            DurationFormatter.durationToString(
                DurationConverter.secondsToDuration(TRACK.tracked ?? 0)
            )
        );

        fixture.componentRef.setInput('track', NEW_TRACK);
        fixture.detectChanges();

        expect(trackedInput.value).toBe(
            DurationFormatter.durationToString(
                DurationConverter.secondsToDuration(NEW_TRACK.tracked ?? 0)
            )
        );
    });

    it('disables the Save button again after an async save resets the form', async () => {
        const fixture = TestBed.createComponent(TrackFormComponent);
        fixture.componentRef.setInput('track', NEW_TRACK);
        fixture.detectChanges();

        const saveButton: HTMLButtonElement =
            fixture.nativeElement.querySelector('ui-button button');
        expect(saveButton).toBeTruthy();
        expect(saveButton.disabled).toBe(true);

        fixture.componentInstance.form.markAsDirty();
        fixture.componentInstance.form.controls.tracked.setValue('1h');
        fixture.componentInstance.form.controls.endAt.setValue(new Date('2026-10-09T12:00:00'));
        fixture.detectChanges();
        expect(saveButton.disabled).toBe(false);

        fixture.componentInstance.onSaveTrack();
        await fixture.whenStable();
        fixture.detectChanges();

        expect(trackerService.insertTrack$).toHaveBeenCalled();
        expect(saveButton.disabled).toBe(true);
    });
});
