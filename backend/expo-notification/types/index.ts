export interface NotificationMessage {
  tokenMap: Record<string, string>;
  title: string;
  subTitle?: string;
  text: string;
  imageURL?: string;
}

// sent to "exponential-backoff-retry", bytes are base64 to match the go and java side
export interface RetryEvent {
  partitionId: string; // "<group>:<topic>:<partition>:<offset>" of the first failed record
  reason: string;
  // set only on the first failure of a record
  backoff?: number;
  multiplier?: number;
  cap?: number;
  maxFailure?: number;
  topic?: string;
  key?: string;
  headers?: Record<string, string>;
  value?: string;
}
