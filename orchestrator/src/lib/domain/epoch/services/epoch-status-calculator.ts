import { Epoch } from '../entities/epoch';
import { EpochStatus } from '../value-objects/epoch-status';
import { IndexType } from '../value-objects/index-type';

/**
 * Domain service for calculating epoch status based on business rules
 */
export class EpochStatusCalculator {
  /**
   * Calculate the appropriate status for an epoch based on its indexes
   */
  calculateStatus(epoch: Epoch): EpochStatus {
    const indexes = epoch.getIndexes();
    const gsfaIndexes = epoch.getGsfaIndexes();
    
    // Get all possible index types
    const allIndexTypes = IndexType.all();
    
    // Count indexes by type
    const indexTypeCounts = new Map<string, number>();
    indexes.forEach(index => {
      const type = index.getType().getValue();
      indexTypeCounts.set(type, (indexTypeCounts.get(type) || 0) + 1);
    });
    
    // Check conditions
    const hasAllRegularIndexes = allIndexTypes.every(
      type => (indexTypeCounts.get(type.getValue()) || 0) > 0
    );
    
    const hasGsfaIndex = gsfaIndexes.some(gsfa => gsfa.exists());
    const hasSomeIndexes = indexTypeCounts.size > 0;
    
    // Apply business rules
    if (!hasSomeIndexes) {
      return EpochStatus.NotProcessed();
    }
    
    if (hasAllRegularIndexes && hasGsfaIndex) {
      return EpochStatus.Complete();
    }
    
    if (hasAllRegularIndexes) {
      return EpochStatus.Indexed();
    }
    
    return EpochStatus.Processing();
  }
  
  /**
   * Check if an epoch should be updated based on new index information
   */
  shouldUpdateStatus(epoch: Epoch): boolean {
    const currentStatus = epoch.getStatus();
    const calculatedStatus = this.calculateStatus(epoch);
    
    return !currentStatus.equals(calculatedStatus) && 
           currentStatus.canTransitionTo(calculatedStatus);
  }
}