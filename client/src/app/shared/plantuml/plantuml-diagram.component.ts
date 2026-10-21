import {
    ChangeDetectionStrategy,
    Component,
    DestroyRef,
    Injector,
    afterNextRender,
    computed,
    effect,
    inject,
    input,
    output,
    signal
} from '@angular/core';
import { SettingsStore } from 'src/app/core/settings/settings.store';
import { PlantUmlConverter } from './converter/plantuml.converter';
import { PlantUmlRenderService, PlantUmlResult } from './plantuml-render.service';

type DiagramState =
    | { kind: 'disabled' }
    | { kind: 'loading' }
    | { kind: 'svg'; svg: string }
    | { kind: 'error'; error: 'syntax' | 'unavailable'; message?: string };

@Component({
    selector: 'app-plantuml-diagram',
    templateUrl: './plantuml-diagram.component.html',
    styleUrls: ['./plantuml-diagram.component.scss'],
    changeDetection: ChangeDetectionStrategy.OnPush,
    standalone: false
})
export class PlantUmlDiagramComponent {
    public readonly source = input.required<string>();

    public readonly debounce = input(0);

    public readonly settled = output<void>();

    private readonly renderService = inject(PlantUmlRenderService);
    private readonly settings = inject(SettingsStore);
    private readonly injector = inject(Injector);

    protected readonly state = signal<DiagramState>({ kind: 'loading' });
    protected readonly dataUrl = computed(() => {
        const st = this.state();
        return st.kind === 'svg' ? PlantUmlConverter.toDataUrl(st.svg) : '';
    });
    protected readonly errorKey = computed(() => {
        const st = this.state();
        return st.kind === 'error' && st.error === 'unavailable'
            ? 'SHARED.PLANTUML.UNAVAILABLE'
            : 'SHARED.PLANTUML.SYNTAX_ERROR';
    });
    protected readonly errorMessage = computed(() => {
        const st = this.state();
        return st.kind === 'error' ? (st.message ?? '') : '';
    });

    private timer: ReturnType<typeof setTimeout> | null = null;
    private attempt = 0;

    public constructor() {
        effect(() => this.applySource(this.source()));
        inject(DestroyRef).onDestroy(() => this.clearTimer());
    }

    private applySource(source: string): void {
        this.clearTimer();
        if (!this.settings.isPlantUmlEnabled()) {
            this.settle({ kind: 'disabled' });
            return;
        }
        const cached = this.renderService.cached(source);
        if (cached) {
            this.settle(this.toState(cached));
            return;
        }
        this.state.set({ kind: 'loading' });
        const attempt = ++this.attempt;
        const delay = this.debounce();
        if (delay > 0) {
            this.timer = setTimeout(() => this.render(source, attempt), delay);
        } else {
            this.render(source, attempt);
        }
    }

    private render(source: string, attempt: number): void {
        this.timer = null;
        void this.renderService.render(source).then(result => {
            // A newer source may have arrived while this render was in flight.
            if (attempt !== this.attempt) {
                return;
            }
            this.settle(this.toState(result));
        });
    }

    private toState(result: PlantUmlResult): DiagramState {
        return 'svg' in result
            ? { kind: 'svg', svg: result.svg }
            : { kind: 'error', error: result.error, message: result.message };
    }

    private settle(state: DiagramState): void {
        this.state.set(state);
        afterNextRender({ write: () => this.settled.emit() }, { injector: this.injector });
    }

    private clearTimer(): void {
        if (this.timer !== null) {
            clearTimeout(this.timer);
            this.timer = null;
        }
    }
}
