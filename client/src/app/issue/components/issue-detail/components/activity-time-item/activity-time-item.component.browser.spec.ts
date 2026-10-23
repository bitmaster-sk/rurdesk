import { CommonModule } from '@angular/common';
import { Component, Directive, input, output, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { By } from '@angular/platform-browser';
import { TranslateModule, TranslateService } from '@ngx-translate/core';
import { of } from 'rxjs';
import en from 'src/assets/i18n/en.json';
import { User } from 'src/app/auth/model/user.model';
import { AuthStore } from 'src/app/auth/store/auth.store';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { TrackerService } from 'src/app/shared/tracker/tracker.service';
import { AvatarStub, TablerIconStub } from 'src/testing/stubs';
import { ActivityTimeItemComponent } from './activity-time-item.component';

const ME: User = { idUser: 7, name: 'Me', email: 'm@m.sk', colorAvatarBg: '#111' };

const track = (over: Partial<Track> = {}): Track => {
    const base: Track = {
        idTrack: 3,
        idUser: 7,
        idIssue: 10,
        idIssuePublic: 142,
        idProject: 2,
        issueTitle: 'Global time tracker',
        tracked: 5400,
        startAt: new Date('2026-09-08T10:00:00Z'),
        endAt: new Date('2026-09-08T11:30:00Z'),
        note: null
    };
    return { ...base, ...over };
};

@Directive({ selector: '[uiConfirm]', standalone: true })
class UiConfirmStub {
    public readonly confirmText = input<string>('');
    public readonly confirmed = output<void>();
}

@Component({ selector: 'app-track-edit-form', template: '', standalone: true })
class TrackEditFormStub {
    public readonly track = input<Track | null>(null);
    public readonly saved = output<Track>();
    public readonly cancelled = output<void>();
}

describe('ActivityTimeItemComponent (browser)', () => {
    let deleteTrack$: ReturnType<typeof vi.fn>;

    beforeEach(async () => {
        deleteTrack$ = vi.fn(() => of(undefined));

        await TestBed.configureTestingModule({
            declarations: [ActivityTimeItemComponent],
            providers: [
                { provide: AuthStore, useValue: { user: signal(ME) } },
                { provide: TrackerService, useValue: { deleteTrack$ } }
            ]
        })
            .overrideComponent(ActivityTimeItemComponent, { set: { template: '' } })
            .compileComponents();
    });

    function create(input: Track) {
        const fixture = TestBed.createComponent(ActivityTimeItemComponent);
        fixture.componentRef.setInput('track', input);
        fixture.detectChanges();
        return fixture.componentInstance as unknown as {
            canEdit: () => boolean;
            onDelete: () => void;
        };
    }

    it('offers editing only on entries the signed-in user logged', () => {
        expect(create(track()).canEdit()).toBe(true);
        expect(create(track({ idUser: 99 })).canEdit()).toBe(false);
    });

    it('deletes the entry', () => {
        create(track()).onDelete();

        expect(deleteTrack$).toHaveBeenCalledWith(3);
    });
});

describe('ActivityTimeItemComponent editing (browser)', () => {
    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [ActivityTimeItemComponent],
            imports: [
                CommonModule,
                TranslateModule.forRoot(),
                TablerIconStub,
                AvatarStub,
                UiConfirmStub,
                TrackEditFormStub
            ],
            providers: [
                { provide: AuthStore, useValue: { user: signal(ME) } },
                {
                    provide: TrackerService,
                    useValue: { deleteTrack$: vi.fn(() => of(undefined)) }
                }
            ]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', en);
        translate.use('en');
    });

    function render(): {
        fixture: ComponentFixture<ActivityTimeItemComponent>;
        el: HTMLElement;
    } {
        const fixture = TestBed.createComponent(ActivityTimeItemComponent);
        fixture.componentRef.setInput('track', track());
        fixture.detectChanges();
        return { fixture, el: fixture.nativeElement as HTMLElement };
    }

    function openEdit(): { fixture: ComponentFixture<ActivityTimeItemComponent>; el: HTMLElement } {
        const page = render();
        (page.el.querySelector('[data-testid="track-edit"]') as HTMLElement).click();
        page.fixture.detectChanges();
        return page;
    }

    it('reveals the edit form on the edit button click', () => {
        const page = render();
        expect(page.el.querySelector('app-track-edit-form')).toBeNull();

        (page.el.querySelector('[data-testid="track-edit"]') as HTMLElement).click();
        page.fixture.detectChanges();

        expect(page.el.querySelector('app-track-edit-form')).not.toBeNull();
    });

    it('hides the edit form after the edited track is saved', () => {
        const page = openEdit();

        page.fixture.debugElement
            .query(By.directive(TrackEditFormStub))
            .injector.get(TrackEditFormStub)
            .saved.emit(track());
        page.fixture.detectChanges();

        expect(page.el.querySelector('app-track-edit-form')).toBeNull();
    });

    it('hides the edit form when the edit is cancelled', () => {
        const page = openEdit();

        page.fixture.debugElement
            .query(By.directive(TrackEditFormStub))
            .injector.get(TrackEditFormStub)
            .cancelled.emit();
        page.fixture.detectChanges();

        expect(page.el.querySelector('app-track-edit-form')).toBeNull();
    });
});
