import { TestBed } from '@angular/core/testing';
import { Router } from '@angular/router';
import { NotFoundPage } from './not-found.page';

describe('NotFoundPage (browser)', () => {
    it('navigates home when the action is triggered', () => {
        const navigateByUrl = vi.fn();
        TestBed.configureTestingModule({
            imports: [NotFoundPage],
            providers: [{ provide: Router, useValue: { navigateByUrl } }]
        })
            // Skip the template (robot SVG + translate pipe + ui-button) — this
            // exercises the navigation behaviour only.
            .overrideComponent(NotFoundPage, { set: { template: '', imports: [] } });

        const comp = TestBed.createComponent(NotFoundPage).componentInstance as unknown as {
            onHome: () => void;
        };
        comp.onHome();

        expect(navigateByUrl).toHaveBeenCalledWith('/');
    });
});
