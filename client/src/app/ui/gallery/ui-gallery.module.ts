import { CommonModule } from '@angular/common';
import { NgModule } from '@angular/core';
import { FormsModule } from '@angular/forms';
import { RouterModule, Routes } from '@angular/router';
import { DragDropModule } from '@angular/cdk/drag-drop';
import {
    IconArrowsSort,
    IconDownload,
    IconFilter,
    IconGripVertical,
    IconLayoutList,
    IconLogout,
    IconMessages,
    IconPlus,
    IconSearch,
    IconSettings,
    IconSortAscending,
    IconSortDescending,
    IconTrash,
    IconUser,
    IconX,
    TablerIconComponent,
    provideTablerIcons
} from '@tabler/icons-angular';
import { UiModule } from '../ui.module';
import { UiGalleryPage } from './ui-gallery.page';

const routes: Routes = [{ path: '', component: UiGalleryPage }];

/**
 * Dev-only gallery of the `ui-*` design-system primitives. Lazy-loaded at `/ui`
 * and excluded from production builds (see app-routing.module.ts).
 */
@NgModule({
    declarations: [UiGalleryPage],
    imports: [
        CommonModule,
        FormsModule,
        UiModule,
        DragDropModule,
        TablerIconComponent,
        RouterModule.forChild(routes)
    ],
    providers: [
        provideTablerIcons({
            IconPlus,
            IconTrash,
            IconDownload,
            IconSearch,
            IconX,
            IconLayoutList,
            IconFilter,
            IconUser,
            IconMessages,
            IconSettings,
            IconLogout,
            IconGripVertical,
            IconArrowsSort,
            IconSortAscending,
            IconSortDescending
        })
    ]
})
export class UiGalleryModule {}
