import { TestBed } from '@angular/core/testing';
import { FormsModule } from '@angular/forms';
import { IssueTypeDropdownComponent } from './issue-type-dropdown.component';
import { IssueType } from 'src/app/issue-type/model/issue-type.model';

const bug: IssueType = {
    idIssueType: 1,
    idProject: 1,
    name: 'Bug',
    protected: false,
    orderRank: 1
};
const feature: IssueType = {
    idIssueType: 2,
    idProject: 1,
    name: 'Feature',
    protected: false,
    orderRank: 2
};

describe('IssueTypeDropdownComponent (browser)', () => {
    function create(multi: boolean, types: IssueType[] | null = [bug, feature]) {
        TestBed.configureTestingModule({
            declarations: [IssueTypeDropdownComponent],
            imports: [FormsModule]
        }).overrideComponent(IssueTypeDropdownComponent, { set: { template: '' } });

        const fixture = TestBed.createComponent(IssueTypeDropdownComponent);
        fixture.componentRef.setInput('multi', multi);
        fixture.componentRef.setInput('issueTypes', types);
        fixture.detectChanges();
        return fixture;
    }

    it('reports the picked id upward in single mode', () => {
        const fixture = create(false);
        const component = fixture.componentInstance as any;
        const seen: unknown[] = [];
        component.registerOnChange((v: unknown) => seen.push(v));

        component.onValueChange(2);

        expect(seen).toEqual([2]);
    });

    it('accepts a written id in single mode', () => {
        const fixture = create(false);
        const component = fixture.componentInstance as any;

        component.writeValue(1);

        expect(component.value()).toBe(1);
    });

    it('coalesces a written null to no selection', () => {
        const fixture = create(false);
        const component = fixture.componentInstance as any;

        component.writeValue(null);

        expect(component.value()).toBeNull();
    });

    it('reports ids, not objects, upward in multi mode', () => {
        const fixture = create(true);
        const component = fixture.componentInstance as any;
        const seen: unknown[] = [];
        component.registerOnChange((v: unknown) => seen.push(v));

        component.onMultiValueChange([bug, feature]);

        expect(seen).toEqual([[1, 2]]);
    });

    it('resolves ids written before the options arrive', () => {
        const fixture = create(true, []);
        const component = fixture.componentInstance as any;

        component.writeValue([2]);
        fixture.detectChanges();
        expect(component.multiValue()).toEqual([]);

        fixture.componentRef.setInput('issueTypes', [bug, feature]);
        fixture.detectChanges();
        expect(component.multiValue()).toEqual([feature]);
    });

    it('treats a null option list as empty instead of throwing', () => {
        const fixture = create(true, null);
        const component = fixture.componentInstance as any;

        component.writeValue([1]);
        fixture.detectChanges();

        expect(component.multiValue()).toEqual([]);
    });
});
