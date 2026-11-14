import { SsoConverter } from 'src/app/auth/converter/sso.converter';
import { SsoTestResult } from '../model/admin-sso.model';
import { SsoTestAction } from '../model/sso-test-action.enum';
import { SsoTestError } from '../model/sso-test-error.enum';
import { SsoWarning } from '../model/sso-warning.enum';

export abstract class SsoTestResultConverter {
    private static readonly errors: ReadonlySet<string> = new Set(Object.values(SsoTestError));
    private static readonly warnings: ReadonlySet<string> = new Set(Object.values(SsoWarning));
    private static readonly actions: ReadonlySet<string> = new Set(Object.values(SsoTestAction));

    public static toErrorKey(error: string | undefined): string {
        return `ADMIN.SSO.TEST_ERROR.${SsoTestResultConverter.known(error, SsoTestResultConverter.errors)}`;
    }

    public static toWarningKeys(warnings: readonly string[]): string[] {
        return warnings.map(
            warning =>
                `ADMIN.SSO.WARNING.${SsoTestResultConverter.known(warning, SsoTestResultConverter.warnings)}`
        );
    }

    public static toOutcomeKey(action: SsoTestAction | undefined): string {
        return `ADMIN.SSO.OUTCOME.${SsoTestResultConverter.known(action, SsoTestResultConverter.actions)}`;
    }

    public static toReasonKey(result: Pick<SsoTestResult, 'action' | 'reason'>): string | null {
        return result.action === SsoTestAction.Reject
            ? SsoConverter.toErrorKey(result.reason ?? '')
            : null;
    }

    private static known(value: string | undefined, values: ReadonlySet<string>): string {
        return value !== undefined && values.has(value) ? value.toUpperCase() : 'UNKNOWN';
    }
}
