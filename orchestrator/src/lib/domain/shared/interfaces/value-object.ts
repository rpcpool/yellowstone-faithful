/**
 * Base interface for value objects
 */
export interface ValueObject<T> {
  equals(other: ValueObject<T>): boolean;
  getValue(): T;
}