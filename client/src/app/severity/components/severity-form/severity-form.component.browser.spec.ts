import { TestBed } from '@angular/core/testing';
import { ReactiveFormsModule } from '@angular/forms';
import { TranslateModule } from '@ngx-translate/core';
import { SEVERITY_COLORS } from '../../constants/severity-colors';
import { IssueSeverity } from '../../model/issue-severity.model';
import { SeverityFormComponent } from './severity-form.component';

describe('SeverityFormComponent', () => {
    beforeEach(() => {
        TestBed.configureTestingModule({
            declarations: [SeverityFormComponent],
            imports: [ReactiveFormsModule, TranslateModule.forRoot()]
        }).overrideComponent(SeverityFormComponent, { set: { template: '' } });
    });

    function create(severity?: IssueSeverity) {
        const fixture = TestBed.createComponent(SeverityFormComponent);
        fixture.componentRef.setInput('severity', severity ?? {});
        fixture.detectChanges();
        return fixture;
    }

    it('starts a new severity on a palette colour so the form is savable once titled', () => {
        const fixture = create();

        expect(SEVERITY_COLORS).toContain(fixture.componentInstance.form.value.color);

        fixture.componentInstance.form.patchValue({ title: 'Blocker' });
        expect(fixture.componentInstance.form.valid).toBe(true);
    });

    it('keeps the stored colour when editing', () => {
        const severity: IssueSeverity = {
            idSeverity: 1,
            idProject: 10,
            title: 'High',
            color: '#d32f2f',
            protected: false,
            orderRank: 1
        };
        const fixture = create(severity);

        expect(fixture.componentInstance.form.value.color).toBe('#d32f2f');
    });
});
