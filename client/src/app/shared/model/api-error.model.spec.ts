import { describe, expect, it } from 'vitest';
import { ApiError } from './api-error.model';

describe('ApiError', () => {
    describe('translateKeyOf', () => {
        it('returns the translateKey when all fields are present and valid', () => {
            const error = {
                error: { code: 'NOT_FOUND', message: 'gone', translateKey: 'error.not_found' }
            };
            expect(ApiError.translateKeyOf(error)).toBe('error.not_found');
        });

        it('returns the translateKey even when code is a number (wrong type)', () => {
            const error = {
                error: { code: 404, message: 'gone', translateKey: 'error.not_found' }
            };
            expect(ApiError.translateKeyOf(error)).toBe('error.not_found');
        });

        it('returns the translateKey even when code is missing', () => {
            const error = { error: { message: 'gone', translateKey: 'error.not_found' } };
            expect(ApiError.translateKeyOf(error)).toBe('error.not_found');
        });

        it('returns null when translateKey is missing', () => {
            const error = { error: { code: 'NOT_FOUND', message: 'gone' } };
            expect(ApiError.translateKeyOf(error)).toBeNull();
        });

        it('returns null when translateKey is not a string', () => {
            const error = { error: { code: 'NOT_FOUND', message: 'gone', translateKey: 42 } };
            expect(ApiError.translateKeyOf(error)).toBeNull();
        });

        it('returns null when error is not a record', () => {
            expect(ApiError.translateKeyOf('oops')).toBeNull();
            expect(ApiError.translateKeyOf(null)).toBeNull();
            expect(ApiError.translateKeyOf(undefined)).toBeNull();
        });

        it('returns null when error.error is not a record', () => {
            const error = { error: 'string-body' };
            expect(ApiError.translateKeyOf(error)).toBeNull();
        });

        it('returns null when error.error is missing', () => {
            const error = { status: 500 };
            expect(ApiError.translateKeyOf(error)).toBeNull();
        });
    });

    describe('bodyOf', () => {
        it('returns the body when all fields are valid strings', () => {
            const error = { error: { code: 'X', message: 'm', translateKey: 'k' } };
            expect(ApiError.bodyOf(error)).toEqual({ code: 'X', message: 'm', translateKey: 'k' });
        });

        it('returns null when code is a number (full-body guard stays strict)', () => {
            const error = { error: { code: 42, message: 'm', translateKey: 'k' } };
            expect(ApiError.bodyOf(error)).toBeNull();
        });
    });
});
