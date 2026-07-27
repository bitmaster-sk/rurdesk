import { ComponentFixture, TestBed } from '@angular/core/testing';
import { ProjectBuilderStepSuccessComponent } from './project-builder-step-success.component';
import { TranslateModule } from '@ngx-translate/core';
import { ChangeDetectionStrategy } from '@angular/core';
import { UiModule } from '../../../ui/ui.module';
import { TablerIconComponent, provideTablerIcons, IconList } from '@tabler/icons-angular';

describe('ProjectBuilderStepSuccessComponent', () => {
    let fixture: ComponentFixture<ProjectBuilderStepSuccessComponent>;
    let component: ProjectBuilderStepSuccessComponent;

    beforeEach(async () => {
        await TestBed.configureTestingModule({
            declarations: [ProjectBuilderStepSuccessComponent],
            imports: [TranslateModule.forRoot(), UiModule, TablerIconComponent],
            providers: [provideTablerIcons({ IconList })]
        })
            .overrideComponent(ProjectBuilderStepSuccessComponent, {
                set: { changeDetection: ChangeDetectionStrategy.Default }
            })
            .compileComponents();

        fixture = TestBed.createComponent(ProjectBuilderStepSuccessComponent);
        component = fixture.componentInstance;
        fixture.componentRef.setInput('createdCount', 5);
        fixture.detectChanges();
    });

    it('emits goToIssues when onGoToIssues is called', () => {
        let emittedCount = 0;
        component.goToIssues.subscribe(() => emittedCount++);
        component.onGoToIssues();
        expect(emittedCount).toBe(1);
    });
});
