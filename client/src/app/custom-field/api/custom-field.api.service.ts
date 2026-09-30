import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable, inject } from '@angular/core';
import { Observable } from 'rxjs';
import { CustomField } from '../model/custom-field.model';
import { CustomFieldUsage } from '../model/custom-field-usage.model';

export type CustomFieldOptionIntent =
    { migrateOptionTo: number } | { deleteOptionValues: true } | undefined;

@Injectable({
    providedIn: 'root'
})
export class CustomFieldApi {
    private http = inject(HttpClient);

    public load$(): Observable<CustomField[]> {
        return this.http.get<CustomField[]>(`/api/private/custom-field`);
    }

    public insert$(
        field: Partial<CustomField>,
        backfill?: string | number | boolean | null
    ): Observable<CustomField> {
        const body: Record<string, unknown> = { ...field };
        if (backfill !== undefined && backfill !== null) {
            body['backfill'] = backfill;
        }
        return this.http.post<CustomField>(`/api/private/custom-field`, body);
    }

    // Never send the whole entity: isArchived: false would ride along on a rename and
    // unarchive the field, and "options untouched" would become "options replaced".
    public update$(
        field: CustomField,
        opts?: {
            isArchived?: boolean;
            intent?: CustomFieldOptionIntent;
            withOptions?: boolean;
            withDefaultValue?: boolean;
            backfill?: unknown;
        }
    ): Observable<CustomField> {
        let params = new HttpParams();
        const intent = opts?.intent;
        if (intent && 'migrateOptionTo' in intent) {
            params = params.set('migrateOptionTo', String(intent.migrateOptionTo));
        }
        if (intent && 'deleteOptionValues' in intent) {
            params = params.set('deleteOptionValues', 'true');
        }

        const body: Record<string, unknown> = {
            idProject: field.idProject,
            name: field.name,
            key: field.key,
            fieldType: field.fieldType,
            isRequired: field.isRequired,
            orderRank: field.orderRank
        };
        if (opts?.isArchived !== undefined) {
            body['isArchived'] = opts.isArchived;
        }
        if (opts?.withOptions) {
            body['options'] = field.options;
        }
        // Sending this on an archive or a reorder would clear the stored default.
        if (opts?.withDefaultValue) {
            body['defaultValue'] = field.defaultValue;
        }
        if (opts?.backfill !== undefined && opts.backfill !== null) {
            body['backfill'] = opts.backfill;
        }

        return this.http.patch<CustomField>(
            `/api/private/custom-field/${field.idCustomField}`,
            body,
            { params }
        );
    }

    public archive$(field: CustomField, isArchived: boolean): Observable<CustomField> {
        return this.update$(field, { isArchived });
    }

    public usage$(idProject: number, idCustomField: number): Observable<CustomFieldUsage> {
        return this.http.get<CustomFieldUsage>(
            `/api/private/custom-field/${idCustomField}/project/${idProject}/usage`
        );
    }

    public delete$(
        idProject: number,
        idCustomField: number,
        deleteValues = false
    ): Observable<void> {
        let params = new HttpParams();
        if (deleteValues) {
            params = params.set('deleteValues', 'true');
        }
        return this.http.delete<void>(
            `/api/private/custom-field/${idCustomField}/project/${idProject}`,
            { params }
        );
    }
}
