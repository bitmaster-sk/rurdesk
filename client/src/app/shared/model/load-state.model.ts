export interface LoadState<T> {
    isLoading: boolean;
    data: T | null;
}

export abstract class LoadStateUtil {
    public static idle<T>(): LoadState<T> {
        return { isLoading: false, data: null };
    }

    public static loading<T>(previous: T | null = null): LoadState<T> {
        return { isLoading: true, data: previous };
    }

    public static loaded<T>(data: T | null): LoadState<T> {
        return { isLoading: false, data };
    }
}
