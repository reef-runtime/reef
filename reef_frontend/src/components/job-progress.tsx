import { FC } from 'react';
import { cn } from '@/lib/utils';

import { IJob, IJobStatus } from '@/types/job';
import { Progress } from './ui/progress';

interface JobProgressProps {
  job?: IJob | null;
  progress?: number;
  className?: string;
}

const JobProgress: FC<JobProgressProps> = ({
  className,
  job,
  progress,
  ...props
}) => {
  // if neither is defined return null
  if (!job && progress === undefined) return null;

  // if job is passed in only show for running jobs with progress
  if (
    job &&
    (job?.status !== IJobStatus.StatusRunning || job.progress < 0.0001)
  )
    return null;

  const percentage = Math.floor((job?.progress ?? progress ?? 0) * 100);

  return (
    <Progress
      value={percentage}
      className={cn('h-1.5 w-full bg-muted/90', className)}
      {...props}
    />
  );
};

export default JobProgress;
