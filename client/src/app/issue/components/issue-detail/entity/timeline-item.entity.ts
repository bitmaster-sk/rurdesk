import { Message } from 'src/app/message/model/message.model';
import { Track } from 'src/app/shared/tracker/model/track.model';

export type TimelineItemType = 'comment' | 'time';

export interface CommentTimelineItem {
    type: 'comment';
    date: Date;
    data: Message;
}

export interface TimeTimelineItem {
    type: 'time';
    date: Date;
    data: Track;
}

export type TimelineItem = CommentTimelineItem | TimeTimelineItem;

export interface TimelineDisplayItem {
    item: TimelineItem;
    showSeparator: boolean;
    dateLabel: string;
}
