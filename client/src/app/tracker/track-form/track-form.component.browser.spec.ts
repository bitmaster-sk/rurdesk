import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { DurationConverter } from 'src/app/shared/duration/duration.converter';
import { DurationFormatter } from 'src/app/shared/duration/duration.formatter';
import { TrackForm } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { TrackFormComponent } from './track-form.component';

const TRACK: TrackForm = {
    idTrack: 5,
    idIssue: 10,
    idUser: 7,
    tracked: 3661,
    endAt: new Date('2026-10-09T10:00:00'),
    note: null
};

describe('TrackFormComponent (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [TrackFormComponent],
            imports: [ReactiveFormsModule],
            providers: [
                {
                    provide: TrackerService,
                    useValue: {
                        insertTrack$: () => of(undefined),
                        updateTrack$: () => of(undefined)
                    }
                }
            ]
        })
            .overrideComponent(TrackFormComponent, { set: { template: '' } })
            .compileComponents();
    });

    it('re-renders the form when the track input changes', () => {
        const fixture = TestBed.createComponent(TrackFormComponent);
        fixture.componentRef.setInput('track', TRACK);
        fixture.detectChanges();

        expect(fixture.componentInstance.form.value.idTrack).toBe(5);
        expect(fixture.componentInstance.form.value.idIssue).toBe(10);
        expect(fixture.componentInstance.form.value.idUser).toBe(7);

        fixture.componentRef.setInput('track', {
            ...TRACK,
            idTrack: 99,
            tracked: 60
        });
        fixture.detectChanges();

        expect(fixture.componentInstance.form.value.idTrack).toBe(99);
        expect(fixture.componentInstance.form.value.tracked).toBe(
            DurationFormatter.durationToString(DurationConverter.secondsToDuration(60))
        );
    });
});
