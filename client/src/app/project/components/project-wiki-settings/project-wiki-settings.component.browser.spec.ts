import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { of } from 'rxjs';
import { describe, expect, it, vi } from 'vitest';
import { WikiApi } from 'src/app/wiki/api/wiki.api.service';
import { ProjectWikiSettingsComponent } from './project-wiki-settings.component';

describe('ProjectWikiSettingsComponent', () => {
    async function setup(): Promise<{
        input: HTMLInputElement;
        updateSettings$: ReturnType<typeof vi.fn>;
    }> {
        const updateSettings$ = vi.fn().mockReturnValue(of(undefined));
        await TestBed.configureTestingModule({
            imports: [ReactiveFormsModule],
            declarations: [ProjectWikiSettingsComponent],
            providers: [
                {
                    provide: WikiApi,
                    useValue: {
                        loadTree$: () =>
                            of({ spaces: [], nodes: [], alwaysTokens: 3200, tokenLimit: 12000 }),
                        updateSettings$
                    }
                }
            ]
        })
            .overrideComponent(ProjectWikiSettingsComponent, {
                set: { template: `<input type="number" [formControl]="tokenLimit" />` }
            })
            .compileComponents();
        const fixture = TestBed.createComponent(ProjectWikiSettingsComponent);
        fixture.componentRef.setInput('project', { idProject: 7 });
        fixture.detectChanges();
        await fixture.whenStable();
        return {
            input: (fixture.nativeElement as HTMLElement).querySelector('input')!,
            updateSettings$
        };
    }

    function type(input: HTMLInputElement, value: string): void {
        input.value = value;
        input.dispatchEvent(new Event('input'));
        input.dispatchEvent(new Event('blur'));
    }

    it('shows the current limit and saves a changed one when the field is left', async () => {
        const { input, updateSettings$ } = await setup();
        expect(input.value).toBe('12000');

        type(input, '20000');

        expect(updateSettings$).toHaveBeenCalledWith(7, 20000);
    });

    it('does not save when the value did not change or is out of range', async () => {
        const { input, updateSettings$ } = await setup();

        type(input, '12000');
        type(input, '-5');

        expect(updateSettings$).not.toHaveBeenCalled();
    });
});
