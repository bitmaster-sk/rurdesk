import { TestBed } from '@angular/core/testing';
import { ActivatedRoute, Router, convertToParamMap } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { of } from 'rxjs';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { TagStore } from 'src/app/tag/store/tag.store';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ReleaseApi } from '../../api/release.api.service';
import { ReleaseStatus } from '../../constants/release-status.enum';
import { ReleaseStep } from '../../constants/release-step.enum';
import { ReleaseNewPage } from './release-new.page';

describe('ReleaseNewPage', () => {
    let api: Record<string, ReturnType<typeof vi.fn>>;
    let navigate: ReturnType<typeof vi.fn>;
    let loadTags: ReturnType<typeof vi.fn>;

    beforeEach(() => {
        loadTags = vi.fn();
        api = {
            load$: vi
                .fn()
                .mockReturnValue(
                    of([{ idRelease: 7, idTag: 10, name: 'v2', status: ReleaseStatus.Draft }])
                ),
            insert$: vi.fn().mockReturnValue(of({ idRelease: 8 }))
        };
        navigate = vi.fn().mockResolvedValue(true);
    });

    async function setup() {
        await TestBed.configureTestingModule({
            imports: [TranslateModule.forRoot()],
            declarations: [ReleaseNewPage],
            providers: [
                { provide: ReleaseApi, useValue: api },
                {
                    provide: ActivatedRoute,
                    useValue: { snapshot: { paramMap: convertToParamMap({ idProject: '3' }) } }
                },
                { provide: Router, useValue: { navigate } },
                { provide: I18nService, useValue: { instant: (key: string) => key } },
                {
                    provide: TagStore,
                    useValue: {
                        load: loadTags,
                        tagsByProject$: () =>
                            of([
                                { idTag: 9, name: 'next', color: '#000', issueCount: 3 },
                                { idTag: 10, name: 'later', color: '#000', issueCount: 1 }
                            ])
                    }
                }
            ]
        })
            .overrideComponent(ReleaseNewPage, { set: { template: '' } })
            .compileComponents();
        const fixture = TestBed.createComponent(ReleaseNewPage);
        fixture.detectChanges();
        return fixture.componentInstance;
    }

    it('reloads the tags so their task counts are current', async () => {
        await setup();

        expect(loadTags).toHaveBeenCalledTimes(1);
    });

    it('starts a draft and opens it on the tasks step', async () => {
        const page = await setup();

        page['form'].setValue({ idTag: 9, name: ' v1.0.4 ' });
        page['onCreate']();

        expect(api.insert$).toHaveBeenCalledWith(3, { idTag: 9, name: 'v1.0.4' });
        expect(navigate).toHaveBeenCalledWith(['/project', 3, 'release', 8], {
            queryParams: { step: ReleaseStep.Tasks },
            replaceUrl: true
        });
    });

    it('points to the draft a tag already has instead of starting another', async () => {
        const page = await setup();

        page['form'].setValue({ idTag: 10, name: 'v3' });

        expect(page['existingDraft']()?.idRelease).toBe(7);
        expect(page['isValid']()).toBe(false);
        page['onCreate']();
        expect(api.insert$).not.toHaveBeenCalled();
    });
});
