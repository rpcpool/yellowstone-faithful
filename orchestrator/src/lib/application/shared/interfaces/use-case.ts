/**
 * Base interface for application use cases
 */
export interface UseCase<TRequest, TResponse> {
  execute(request: TRequest): Promise<TResponse>;
}