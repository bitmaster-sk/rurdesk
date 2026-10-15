export interface UiSelectDock<T> {
    option: T;
    kind: string;
}

export class UiSelectOptionPicked<T> {
    public readonly option: T;
    public readonly value: unknown;
    public readonly dock: UiSelectDock<T> | null;
    public readonly isDockedOption: boolean;
    public readonly originalEvent?: Event;
    private isPrevented = false;

    public constructor(
        option: T,
        value: unknown,
        dock: UiSelectDock<T> | null,
        isDockedOption: boolean,
        originalEvent?: Event
    ) {
        this.option = option;
        this.value = value;
        this.dock = dock;
        this.isDockedOption = isDockedOption;
        this.originalEvent = originalEvent;
    }

    public get isDefaultPrevented(): boolean {
        return this.isPrevented;
    }

    public preventDefault(): void {
        this.isPrevented = true;
    }
}
