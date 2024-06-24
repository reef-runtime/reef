'use client';

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card';
import { useNodes } from '@/stores/nodes.store';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useJobs } from '@/stores/job.store';
import classNames from 'classnames';
import { IJob, IJobResultContentType, IJobStatus } from '@/types/job';
import { BanIcon, CogIcon } from 'lucide-react';
import JobStatusIcon from '@/components/job-status';
import JobListItem from '@/components/job-list-item';
import { useEffect, useState } from 'react';
import { useLogs } from '@/stores/log.store';
import { ILogEntry, ILogKind } from '@/types/log';
import { Button } from '@/components/ui/button';
import { Textarea } from '@/components/ui/textarea';
import {
  Select,
  SelectTrigger,
  SelectValue,
  SelectContent,
  SelectGroup,
  SelectLabel,
  SelectItem,
} from '@/components/ui/select';
import { set, useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { z } from 'zod';
import {
  Form,
  FormField,
  FormItem,
  FormLabel,
  FormControl,
  FormDescription,
  FormMessage,
} from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { ToastAction } from '@/components/ui/toast';
import { useToast } from '@/components/ui/use-toast';
import { Editor } from './editor';

const schema = z.object({
  name: z.string().min(2).max(50),
  language: z.enum(['c', 'rust']),
  sourceCode: z.string(),
});

export default function Page() {
  const { toast } = useToast();
  const [response, setResponse] = useState<string>('');

  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: 'test-job-name',
      language: 'c',
      sourceCode: '',
    },
  });

  const onSubmit = async (values: z.infer<typeof schema>) => {
    console.log(values);

    const res = await fetch('/api/jobs/submit', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(values),
    });

    toast({
      title: res.status == 200 ? 'Success' : 'Error',
      description: 'Check console for more information',
    });

    setResponse(await res.text());
  };

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="flex flex-col xl:flex-row p-4 space-y-4 xl:space-y-0 xl:space-x-4"
      >
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4 grow">
          <Card className="grid w-full gap-2 col-span-3 ">
            <CardHeader>
              <CardTitle>Code Editor</CardTitle>
              {/* TODO: use bundeled default files + a selector (like for sprache.hpi.church) */}
            </CardHeader>

            <CardContent style={{ height: '85vh' }}>
              <FormField
                control={form.control}
                name="sourceCode"
                render={({ field }) => (
                  <FormItem style={{ height: '100%' }}>
                    <FormControl>
                      <Editor
                        code={
                          '#include "reef.h"\n\nvoid run(uint8_t *dataset, size_t len) {}'
                        }
                        className="editor"
                        onSourceChange={field.onChange}
                      ></Editor>
                    </FormControl>
                  </FormItem>
                )}
              />
            </CardContent>
          </Card>
        </div>

        <div className="w-[300px] flex flex-col space-y-4">
          <Card className="w-[300px] flex flex-col">
            <CardHeader>
              <CardTitle>Job Submission</CardTitle>
            </CardHeader>
            <CardContent>
              <FormField
                control={form.control}
                name="name"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Job Name</FormLabel>
                    <FormControl>
                      <Input placeholder="Job Name" {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name="language"
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Language</FormLabel>
                    <FormControl>
                      <Select onValueChange={field.onChange} defaultValue={'c'}>
                        <SelectTrigger className="w-full">
                          <SelectValue placeholder="Select a language" />
                        </SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            <SelectItem value="c">C</SelectItem>
                            <SelectItem value="rust">Rust</SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </CardContent>
            <div className="grow" />
            <Button type="submit">Submit Job</Button>
          </Card>
          {response && response != '' && (
            <Card className="grid w-full gap-2 col-span-3 ">
              <CardHeader>
                <CardTitle>Response</CardTitle>
              </CardHeader>
              <CardContent>
                <Textarea value={response} className="h-[500px]" />
              </CardContent>
            </Card>
          )}
        </div>
      </form>
    </Form>
  );
}
