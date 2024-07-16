import { FC } from 'react';
import { CogIcon, BanIcon } from 'lucide-react';
import classNames from 'classnames';
import { IJob, IJobStatus } from '@/types/job';

interface JobStatusIconProps {
  job?: IJob | null;
}

export function colorClassForJob(job?: IJob | null): string {
  if (!job) {
    return 'text-gray-300';
  }

  switch (job.status) {
    case IJobStatus.StatusQueued:
      return 'bg-gray-400';
    case IJobStatus.StatusStarting:
      return 'bg-sky-500';
    case IJobStatus.StatusRunning:
      return 'bg-orange-400';
    case IJobStatus.StatusDone:
      return job.result?.success ? 'bg-green-500' : 'bg-red-500';
    default:
      throw 'New job status was added without updating this code';
  }
}

const JobStatusIcon: FC<JobStatusIconProps> = ({ job }) => {
  const color = colorClassForJob(job);

  if (!job) {
    return (
      <div className="w-5 h-5 flex flex-col justify-center items-center">
        <BanIcon className={`h-4 w-4 ${color}`} />
      </div>
    );
  }

  if (job?.status === IJobStatus.StatusStarting) {
    return (
      <div className="w-5 h-5 flex flex-col justify-center items-center">
        <span className="relative flex h-3 w-3">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-sky-400 opacity-75"></span>
          <span
            className={`relative inline-flex rounded-full h-3 w-3 ${color}`}
          ></span>
        </span>
      </div>
    );
  }

  if (job?.status === IJobStatus.StatusRunning) {
    return (
      <div className="w-5 h-5 flex flex-col justify-center items-center">
        <CogIcon
          className={`h-5 w-5 animate-spin-slow ${color.replaceAll('bg', 'text')}`}
        />
      </div>
    );
  }

  return (
    <div className="w-5 h-5 flex flex-col justify-center items-center">
      <span className={`w-3 h-3 rounded-full ${color}`}></span>
    </div>
  );
};

export default JobStatusIcon;
