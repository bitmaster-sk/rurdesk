import { Injectable, inject } from '@angular/core';
import { Router } from '@angular/router';
import { TranslateService } from '@ngx-translate/core';
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
    private readonly translate = inject(TranslateService);
    private readonly t: Translator = (key, params) => this.translate.instant(key, params);

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
            (path, queryParams) => this.router.navigate(path as unknown[], { queryParams }),
            this.t
        );
    }
}
