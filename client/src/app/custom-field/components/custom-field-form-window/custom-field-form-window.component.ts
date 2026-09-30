import { ChangeDetectionStrategy, Component, OnInit, inject, signal } from '@angular/core';
import { HttpErrorResponse } from '@angular/common/http';
import { Project } from 'src/app/project/model/project.model';
import { WindowConfig } from 'src/app/shared/window/entity/window-config';
import { WindowReference } from 'src/app/shared/window/window.reference';
import { CustomField } from '../../model/custom-field.model';
import { CustomFieldApi } from '../../api/custom-field.api.service';
import { CustomFieldType } from '../../constants/custom-field-type.enum';
import { CustomFieldStore } from '../../store/custom-field.store';
import { CustomFieldFormValue } from '../custom-field-form/custom-field-form.component';

export interface CustomFieldWindowData {
    project?: Project;
    customField?: CustomField;
}

export interface CustomFieldWindowResult {
    saved?: CustomField;
    /** Set when the server refused because a removed option still has values. */
    optionConflict?: CustomField;
}

@Component({
    selector: 'app-custom-field-form-window',
    templateUrl: './custom-field-form-window.component.html',
    standalone: false,
    changeDetection: ChangeDetectionStrategy.OnPush
})
export class CustomFieldFormWindowComponent implements OnInit {
    private winRef = inject(WindowReference);
    public winCfg = inject<WindowConfig<CustomFieldWindowData>>(WindowConfig);
    private customFieldApi = inject(CustomFieldApi);
    private customFieldStore = inject(CustomFieldStore);

    protected readonly issuesMissing = signal<number | null>(null);

    protected readonly projectFields = signal<CustomField[]>([]);

    public ngOnInit(): void {
        const idProject = this.customField.idProject;
        if (idProject) {
            this.customFieldStore
                .customFieldsByProject$(idProject)
                .subscribe(fields => this.projectFields.set(fields));
            this.customFieldStore.ensureLoaded();
        }

        const field = this.winCfg.data?.customField;
        if (!field?.idCustomField) {
            return;
        }
        this.customFieldApi
            .usage$(field.idProject, field.idCustomField)
            .subscribe(usage => this.issuesMissing.set(usage.issuesMissing));
    }

    public onSave(value: CustomFieldFormValue): void {
        // Sending options for anything but a select field is a 400.
        const saver = value.isNew
            ? this.customFieldApi.insert$(value.field, value.backfill)
            : this.customFieldApi.update$(value.field, {
                  withOptions: value.field.fieldType === CustomFieldType.Select,
                  withDefaultValue: true,
                  backfill: value.backfill
              });
        saver.subscribe({
            next: saved => this.winRef.close({ saved }),
            error: (error: HttpErrorResponse) => {
                // Any other error leaves the window open: ErrorInterceptor already said
                // what was wrong, and closing would look like the save had worked.
                if (error.status === 409 && !value.isNew) {
                    this.winRef.close({ optionConflict: value.field });
                }
            }
        });
    }

    public onCancel(): void {
        this.winRef.close(null);
    }

    public get customField(): Partial<CustomField> {
        return {
            idProject: this.winCfg.data?.project?.idProject,
            ...this.winCfg.data?.customField
        };
    }
}
