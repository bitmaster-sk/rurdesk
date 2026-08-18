/** konfigurácia okna */
export class WindowConfig<TData = Record<string, unknown>> {
    public header?: string;
    public data?: TData;
}

export const WINDOW_DEFAULT_CONFIG: WindowConfig = {};
