import {
    ApplicationRef,
    ComponentFactoryResolver,
    ComponentRef,
    EmbeddedViewRef,
    Injectable,
    Injector,
    Type,
    inject
} from '@angular/core';
import { tap } from 'rxjs/operators';
import { WindowConfig, WINDOW_DEFAULT_CONFIG } from './entity/window-config';
import { WindowInjector } from './window.injector';
import { WindowReference } from './window.reference';
import { WindowComponent } from './window/window.component';
import merge from 'lodash-es/merge';
import cloneDeep from 'lodash-es/cloneDeep';

@Injectable()
export class WindowService {
    private readonly componentFactoryResolver = inject(ComponentFactoryResolver);
    private readonly appRef = inject(ApplicationRef);
    private readonly injector = inject(Injector);

    /** otovorí nové okno */
    public open(componentType: Type<any>, cfg: WindowConfig): WindowReference {
        const injectorExtension = new WeakMap();
        injectorExtension.set(WindowConfig as any, merge(cloneDeep(WINDOW_DEFAULT_CONFIG), cfg));

        const windowRef = new WindowReference();
        injectorExtension.set(WindowReference as any, windowRef);

        const window = this.createWindow(componentType, injectorExtension);

        windowRef.onClose = windowRef.onClose.pipe(tap(() => this.destroyWindow(window)));

        return windowRef;
    }

    /** vytvorí okno */
    private createWindow(
        componentType: Type<any>,
        injectorExtension: WeakMap<any, any>
    ): ComponentRef<WindowComponent> {
        const factory = this.componentFactoryResolver.resolveComponentFactory(WindowComponent);
        const component = factory.create(new WindowInjector(this.injector, injectorExtension));

        this.appRef.attachView(component.hostView);
        document.body.appendChild(
            (component.hostView as EmbeddedViewRef<any>).rootNodes[0] as HTMLElement
        );
        component.instance.contentType = componentType;

        return component;
    }

    /**
     * Destroys the window. Deferred: `ui-dialog` emits `hide` from inside its effect, so
     * destroying the view tree synchronously (closing via ×) would run during change
     * detection and corrupt Angular's internal state ("view[EFFECTS] is not iterable").
     */
    private destroyWindow(window: ComponentRef<WindowComponent>): void {
        queueMicrotask(() => {
            this.appRef.detachView(window.hostView);
            window.destroy();
        });
    }
}
