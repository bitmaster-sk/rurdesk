import { NoticeAction } from '../constant/notice-action.enum';
import { NoticeSubject } from '../constant/notice-subject.enum';

export interface Notice<T> {
    subject: NoticeSubject;
    action: NoticeAction;
    payload: T;
}
