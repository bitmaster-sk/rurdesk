export enum LicenseSeverity {
    None = 'none',
    Info = 'info',
    Warning = 'warning',
    Error = 'error',
    Expired = 'expired'
}

// Severity is None for free instances and while expiry is far away; the other
// fields are then empty.
export interface LicenseState {
    severity: LicenseSeverity;
    daysRemaining: number;
    expiresAt: string | null;
    isDismissible: boolean;
}
