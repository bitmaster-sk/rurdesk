import { Component, signal } from '@angular/core';
import { ComponentFixture, TestBed } from '@angular/core/testing';
import { FormControl, FormsModule, ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { CreateTagReq, Tag } from 'src/app/tag/model/tag.model';
import { TablerIconStub } from 'src/testing/stubs';
import { UiModule } from 'src/app/ui/ui.module';
import { TagChipComponent } from '../tag-chip/tag-chip.component';
import { TagColorPickerComponent } from '../tag-color-picker/tag-color-picker.component';
import { TagInputComponent } from './tag-input.component';

const PALETTE = ['#dc2626', '#ea580c', '#d97706'];

const tag = (idTag: number, name: string): Tag => ({
    idTag,
    idProject: 1,
    name,
    color: '#16a34a',
    orderRank: idTag,
    issueCount: 0
});

const TAGS = [
    tag(1, 'gateway'),
    tag(2, 'next'),
    tag(3, 'needs-design'),
    tag(4, 'wiki'),
    tag(5, 'design')
];

@Component({
    template: `
        <app-tag-input
            [formControl]="control"
            [idProject]="1"
            [tags]="tags"
            [canCreate]="canCreate()"
            [colors]="palette"
            [defaultColor]="palette[0]"
            (createTag)="created.push($event)"
        />
    `,
    standalone: false
})
class HostComponent {
    public readonly control = new FormControl<number[]>([1], { nonNullable: true });
    public readonly tags = TAGS;
    public readonly palette = PALETTE;
    public readonly canCreate = signal(true);
    public readonly created: CreateTagReq[] = [];
}

describe('TagInputComponent (browser)', () => {
    let fixture: ComponentFixture<HostComponent>;
    let host: HostComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            imports: [
                FormsModule,
                ReactiveFormsModule,
                TranslateModule.forRoot(),
                UiModule,
                TablerIconStub
            ],
            declarations: [
                HostComponent,
                TagInputComponent,
                TagChipComponent,
                TagColorPickerComponent
            ]
        }).compileComponents();
        fixture = TestBed.createComponent(HostComponent);
        host = fixture.componentInstance;
        fixture.detectChanges();
    });

    function field(): HTMLInputElement {
        return fixture.nativeElement.querySelector('[data-testid="tag-input-field"]');
    }

    function type(text: string): void {
        field().value = text;
        field().dispatchEvent(new Event('input'));
        fixture.detectChanges();
    }

    function press(key: string): KeyboardEvent {
        const event = new KeyboardEvent('keydown', { key, cancelable: true });
        field().dispatchEvent(event);
        fixture.detectChanges();
        return event;
    }

    function suggestionNames(): string[] {
        return Array.from<HTMLElement>(
            fixture.nativeElement.querySelectorAll('[data-testid="tag-suggestion"]')
        ).map(row => row.querySelector('.tag-chip__name')!.textContent.trim());
    }

    function shownNames(): string[] {
        return Array.from<HTMLElement>(
            fixture.nativeElement.querySelectorAll('[data-testid="tag-input"] > app-tag-chip')
        ).map(chip => chip.textContent.trim());
    }

    it('suggests tags containing the typed text, prefix matches first', () => {
        type('de');

        expect(suggestionNames()).toEqual(['design', 'needs-design']);
    });

    it('does not suggest a tag the task already has', () => {
        type('a');

        expect(suggestionNames()).not.toContain('gateway');
    });

    it('completes the first prefix match inline', () => {
        type('ne');

        expect(fixture.nativeElement.querySelector('.tag-input__ghost').textContent).toBe('next');
    });

    it('adds the highlighted tag on Tab and clears the text', () => {
        type('ne');
        const event = press('Tab');

        expect(event.defaultPrevented).toBe(true);
        expect(host.control.value).toEqual([1, 2]);
        expect(field().value).toBe('');
        expect(shownNames()).toEqual(['gateway', 'next']);
    });

    it('adds the tag picked with the arrow keys on Enter', () => {
        type('ne');
        press('ArrowDown');
        press('Enter');

        expect(host.control.value).toEqual([1, 3]);
    });

    it('lets Tab move focus on when nothing is typed', () => {
        const event = press('Tab');

        expect(event.defaultPrevented).toBe(false);
        expect(host.control.value).toEqual([1]);
    });

    it('offers to create an unknown tag and sends the chosen color', () => {
        type('release-blocker');

        expect(fixture.nativeElement.querySelector('[data-testid="tag-create"]')).not.toBeNull();
        press('ArrowRight');
        press('Enter');

        expect(host.created).toEqual([
            { idProject: 1, name: 'release-blocker', color: PALETTE[1] }
        ]);
        expect(field().value).toBe('');
    });

    it('does not offer to create a tag whose name exists in another case', () => {
        type('WIKI');

        expect(fixture.nativeElement.querySelector('[data-testid="tag-create"]')).toBeNull();
        expect(suggestionNames()).toEqual(['wiki']);
    });

    it('explains instead of offering creation to a user who may not create tags', () => {
        host.canCreate.set(false);
        fixture.detectChanges();
        type('release-blocker');
        press('Tab');

        expect(fixture.nativeElement.querySelector('[data-testid="tag-create"]')).toBeNull();
        expect(fixture.nativeElement.querySelector('.tag-input__hint')).not.toBeNull();
        expect(host.created).toEqual([]);
    });

    it('caps a typed name at the length the server accepts', () => {
        expect(field().maxLength).toBe(40);
    });

    it('removes the last tag on Backspace in an empty field', () => {
        press('Backspace');

        expect(host.control.value).toEqual([]);
    });

    it('removes a tag with its remove button', () => {
        fixture.nativeElement.querySelector('.tag-chip__remove').click();
        fixture.detectChanges();

        expect(host.control.value).toEqual([]);
    });
});
