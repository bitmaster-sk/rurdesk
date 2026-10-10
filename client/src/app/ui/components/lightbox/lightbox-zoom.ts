export interface ZoomState {
    scale: number;
    x: number;
    y: number;
}

export abstract class UiLightboxZoom {
    public static readonly minScale = 1;
    public static readonly maxScale = 8;
    public static readonly step = 1.5;
    public static readonly wheelSpeed = 0.0015;
    public static readonly initial: ZoomState = { scale: 1, x: 0, y: 0 };

    public static zoomAt(state: ZoomState, factor: number, cx: number, cy: number): ZoomState {
        const scale = Math.min(
            UiLightboxZoom.maxScale,
            Math.max(UiLightboxZoom.minScale, state.scale * factor)
        );
        if (scale === UiLightboxZoom.minScale) {
            return UiLightboxZoom.initial;
        }
        const ratio = scale / state.scale;
        return {
            scale,
            x: cx - ratio * (cx - state.x),
            y: cy - ratio * (cy - state.y)
        };
    }

    public static panBy(state: ZoomState, dx: number, dy: number): ZoomState {
        if (state.scale === UiLightboxZoom.minScale) {
            return state;
        }
        return { ...state, x: state.x + dx, y: state.y + dy };
    }

    public static toTransform(state: ZoomState): string {
        return `translate(${state.x}px, ${state.y}px) scale(${state.scale})`;
    }
}
