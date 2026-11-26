import { signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { describe, expect, it } from 'vitest';
import { NotificationStore } from '../../store/notification.store';
import { NotificationMenuComponent } from './notification-menu.component';

async function badge(unreadCount: number): Promise<string> {
    await TestBed.configureTestingModule({
        declarations: [NotificationMenuComponent],
        providers: [{ provide: NotificationStore, useValue: { unreadCount: signal(unreadCount) } }]
    })
        .overrideComponent(NotificationMenuComponent, { set: { template: '' } })
        .compileComponents();
    const fixture = TestBed.createComponent(NotificationMenuComponent);
    fixture.detectChanges();
    return fixture.componentInstance.badge();
}

describe('NotificationMenuComponent (browser)', () => {
    it('shows no count at 0 and caps it at 99+', async () => {
        expect(await badge(0)).toBe('');
        TestBed.resetTestingModule();
        expect(await badge(7)).toBe('7');
        TestBed.resetTestingModule();
        expect(await badge(100)).toBe('99+');
    });
});
