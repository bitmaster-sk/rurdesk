import { describe, it, expect } from 'vitest';
import { TestBed } from '@angular/core/testing';
import { MentionChipComponent } from './mention-chip.component';
import { User } from 'src/app/auth/model/user.model';

const makeUser = (idUser: number, name: string): User => ({
    idUser,
    name,
    email: '',
    colorAvatarBg: ''
});

describe('MentionChipComponent', () => {
    beforeEach(() => {
        TestBed.configureTestingModule({
            declarations: [MentionChipComponent]
        });
    });

    it('renders live name from candidates Map when present', async () => {
        const f = TestBed.createComponent(MentionChipComponent);
        f.componentRef.setInput('idUser', 1);
        f.componentRef.setInput('name', 'Old Name');
        f.componentRef.setInput('candidates', new Map([[1, makeUser(1, 'Live Name')]]));
        f.detectChanges();
        expect(f.nativeElement.textContent).toContain('@Live Name');
    });

    it('falls back to snapshot name when candidate is absent', async () => {
        const f = TestBed.createComponent(MentionChipComponent);
        f.componentRef.setInput('idUser', 99);
        f.componentRef.setInput('name', 'Snapshot');
        f.componentRef.setInput('candidates', new Map());
        f.detectChanges();
        expect(f.nativeElement.textContent).toContain('@Snapshot');
    });

    it('resolves name from candidates given as User[] array', async () => {
        const f = TestBed.createComponent(MentionChipComponent);
        f.componentRef.setInput('idUser', 7);
        f.componentRef.setInput('name', 'Fallback');
        f.componentRef.setInput('candidates', [makeUser(7, 'Array User')]);
        f.detectChanges();
        expect(f.nativeElement.textContent).toContain('@Array User');
    });
});
