import { Message } from 'src/app/message/model/message.model';
import { Track } from 'src/app/shared/tracker/model/track.model';
import { WikiProposal } from 'src/app/wiki/model/wiki-proposal.model';

export type TimelineItemType = 'comment' | 'time' | 'wikiProposal';

export type TimelineFilter = TimelineItemType | 'attachment';

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

export interface WikiProposalTimelineItem {
    type: 'wikiProposal';
    date: Date;
    data: WikiProposal;
}

export type TimelineItem = CommentTimelineItem | TimeTimelineItem | WikiProposalTimelineItem;

export interface TimelineDisplayItem {
    item: TimelineItem;
    showSeparator: boolean;
    dateLabel: string;
}
