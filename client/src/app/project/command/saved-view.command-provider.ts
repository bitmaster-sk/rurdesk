import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { I18nService } from 'src/app/shared/i18n/i18n.service';
import { Observable, of } from 'rxjs';
import {
    Command,
    CommandContext,
    CommandProvider,
    Translator
} from '../../core/command/command.model';
import { SavedViewStore } from '../store/saved-view.store';
import { buildSavedViewCommands } from './saved-view.commands';

@Injectable({ providedIn: 'root' })
export class SavedViewCommandProvider implements CommandProvider {
    private readonly router = inject(Router);
    private readonly store = inject(SavedViewStore);
    private readonly i18n = inject(I18nService);
    private readonly t: Translator = (key, params) => this.i18n.instant(key, params);

    public prime(ctx: CommandContext): Observable<unknown> {
        const idProject = ctx.idProject;
        if (idProject == null || this.store.idLoadedProject() === idProject) {
            return of(null);
        }
        return this.store.load$(idProject);
    }

    public getCommands(ctx: CommandContext): Command[] {
        return buildSavedViewCommands(
            ctx,
            this.store.views(),
            (path, queryParams) => {
                void this.router.navigate(path, { queryParams });
            },
            this.t
        );
    }
}
