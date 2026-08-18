import { Observable, Subject } from 'rxjs';
import { first } from 'rxjs/operators';

/** referencia na okno, zabezpečuje spojenie medzi vnútornou komponentou a komponentou, ktorá okno otvorila */
export class WindowReference<TResult = unknown> {
    public readonly subjectClose: Subject<TResult> = new Subject<TResult>();

    public onClose: Observable<TResult> = this.subjectClose.asObservable().pipe(first());

    /** odošle informáciu o zavretí okna */
    public close(v: TResult): void {
        this.subjectClose.next(v);
    }
}
