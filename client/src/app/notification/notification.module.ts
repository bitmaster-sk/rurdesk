import { NgModule } from '@angular/core';
import { CommonModule } from '@angular/common';
import { RouterModule } from '@angular/router';
import { TranslateModule } from '@ngx-translate/core';
import { SharedModule } from '../shared/shared.module';
import { NotificationMenuComponent } from './components/notification-menu/notification-menu.component';
import { NotificationCenterComponent } from './components/notification-center/notification-center.component';
import { NotificationCardComponent } from './components/notification-card/notification-card.component';
import { UiModule } from '../ui/ui.module';
import {
    TablerIconComponent,
    provideTablerIcons,
    IconBell,
    IconList,
    IconAt,
    IconWorld,
    IconSquareCheck,
    IconMessage,
    IconUserCheck,
    IconCircleDot,
    IconFlag,
    IconUserPlus,
    IconStar,
    IconArchive,
    IconX
} from '@tabler/icons-angular';

@NgModule({
    declarations: [
        NotificationMenuComponent,
        NotificationCenterComponent,
        NotificationCardComponent
    ],
    imports: [
        CommonModule,
        RouterModule,
        TranslateModule,
        SharedModule,
        UiModule,
        TablerIconComponent
    ],
    providers: [
        provideTablerIcons({
            IconBell,
            IconList,
            IconAt,
            IconWorld,
            IconSquareCheck,
            IconMessage,
            IconUserCheck,
            IconCircleDot,
            IconFlag,
            IconUserPlus,
            IconStar,
            IconArchive,
            IconX
        })
    ],
    exports: [NotificationMenuComponent]
})
export class NotificationModule {}
