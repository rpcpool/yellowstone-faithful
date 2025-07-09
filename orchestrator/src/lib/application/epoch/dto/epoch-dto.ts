/**
 * Data Transfer Objects for Epoch operations
 */

export interface EpochDto {
  id: number;
  epoch: string;
  status: string;
  cid?: string;
  createdAt: Date;
  updatedAt: Date;
  indexes?: EpochIndexDto[];
  gsfaIndexes?: EpochGsfaDto[];
}

export interface EpochIndexDto {
  id?: string;
  epoch: string;
  type: string;
  size: string;
  status: string;
  location: string;
  source: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface EpochGsfaDto {
  id?: number;
  epoch: string;
  exists: boolean;
  location: string;
  createdAt: Date;
  updatedAt: Date;
}

export interface CreateEpochDto {
  epochId: number;
}

export interface UpdateEpochStatusDto {
  epochId: number;
  status: string;
}

export interface AddEpochIndexDto {
  epochId: number;
  type: string;
  size: number;
  status: string;
  location: string;
  source: string;
}

export interface GetEpochsDto {
  page?: number;
  pageSize?: number;
  search?: string;
  status?: string;
}

export interface GetEpochsResponseDto {
  epochs: EpochDto[];
  pagination: {
    page: number;
    pageSize: number;
    totalCount: number;
    totalPages: number;
  };
}

export interface RefreshEpochDto {
  epochId: number;
  sources?: string[];
}