import { FC } from 'react';
import { cn } from '@/lib/utils';

import { IJob, IJobStatus } from '@/types/job';
import { Progress } from './ui/progress';

interface JobProgressProps {
  job?: IJob | null;
  className?: string;
}

const JobProgress: FC<JobProgressProps> = ({ className, job, ...props }) => {
  if (!job || job?.status !== IJobStatus.StatusRunning || job.progress < 0.01)
    return null;

  const percentage = Math.floor(job.progress * 100);

  return (
    <Progress
      value={percentage}
      className={cn('h-1.5 w-full bg-muted/90', className)}
    />
  );
};

export default JobProgress;
