import { describe, it, expect, vi, beforeEach } from 'vitest';
import { BehaviorSubject } from 'rxjs';
import { TestBed } from '@angular/core/testing';
import { ProjectLayoutComponent } from './project-layout.component';
import { AclStore } from '../../store/acl.store';
import { ProjectStore } from '../../project.store';
import { CommandPaletteService } from '../../../core/command/command-palette.service';

describe('ProjectLayoutComponent palette context', () => {
    let project$: BehaviorSubject<any>;
    let setContext: any;
    beforeEach(() => {
        project$ = new BehaviorSubject<any>({ idProject: 42 });
        setContext = vi.fn();
        TestBed.configureTestingModule({
            declarations: [ProjectLayoutComponent],
            providers: [
                { provide: AclStore, useValue: { setRole: vi.fn() } },
                { provide: ProjectStore, useValue: { project$ } },
                { provide: CommandPaletteService, useValue: { setContext } }
            ]
        }).overrideComponent(ProjectLayoutComponent, { set: { template: '' } });
    });

    it('sets the project context from project$', () => {
        TestBed.createComponent(ProjectLayoutComponent).detectChanges();
        expect(setContext).toHaveBeenCalledWith({ idProject: 42, issue: null });
    });

    it('resets the context to null on destroy', () => {
        const f = TestBed.createComponent(ProjectLayoutComponent);
        f.detectChanges();
        setContext.mockClear();
        f.destroy();
        expect(setContext).toHaveBeenCalledWith({ idProject: null, issue: null });
    });
});
