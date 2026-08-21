import { ComponentFixture, TestBed } from '@angular/core/testing';
import { TranslateModule, TranslateService } from '@ngx-translate/core';

import { MrStatusPillComponent } from './mr-status-pill.component';
import { UiModule } from 'src/app/ui/ui.module';
import { CiStatus, MrState, MrStatus } from 'src/app/project/model/git-integration.model';

describe('MrStatusPillComponent', () => {
    let fixture: ComponentFixture<MrStatusPillComponent>;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot(), UiModule],
            declarations: [MrStatusPillComponent]
        }).compileComponents();

        const translate = TestBed.inject(TranslateService);
        translate.setTranslation('en', {
            MR: {
                STATE: { OPEN: 'Open', MERGED: 'Merged', CLOSED: 'Closed' },
                CI: {
                    SUCCESS: 'CI passed',
                    FAILED: 'CI failed',
                    PENDING: 'CI pending',
                    CANCELED: 'CI canceled',
                    SKIPPED: 'CI skipped',
                    UNKNOWN: 'CI unknown'
                },
                APPROVED: 'Approved'
            }
        });
        translate.use('en');

        fixture = TestBed.createComponent(MrStatusPillComponent);
    });

    function renderWithCiStatus(ciStatus: CiStatus): string {
        const status: MrStatus = { state: MrState.Open, approved: false, ciStatus };
        fixture.componentRef.setInput('status', status);
        fixture.detectChanges();
        return (fixture.nativeElement as HTMLElement).textContent ?? '';
    }

    it('labels a canceled pipeline instead of falling back to unknown', () => {
        expect(renderWithCiStatus(CiStatus.Canceled)).toContain('CI canceled');
    });

    it('labels a skipped pipeline instead of falling back to unknown', () => {
        expect(renderWithCiStatus(CiStatus.Skipped)).toContain('CI skipped');
    });

    it('keeps labelling the states that already worked', () => {
        expect(renderWithCiStatus(CiStatus.Success)).toContain('CI passed');
        expect(renderWithCiStatus(CiStatus.Failed)).toContain('CI failed');
        expect(renderWithCiStatus(CiStatus.Pending)).toContain('CI pending');
    });

    it('claims nothing about CI when the host reported nothing', () => {
        expect(renderWithCiStatus(CiStatus.Unknown)).not.toContain('CI');
    });

    it('shows the MR state even when CI is hidden', () => {
        expect(renderWithCiStatus(CiStatus.Unknown)).toContain('Open');
    });
});
