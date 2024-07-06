'use client';

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { useNodes } from '@/stores/nodes.store';
import { Separator } from '@/components/ui/separator';
import { ScrollArea } from '@/components/ui/scroll-area';
import { useJobs } from '@/stores/job.store';
import classNames from 'classnames';
import { IJob, IJobResultContentType, IJobStatus } from '@/types/job';
import { BanIcon, CogIcon } from 'lucide-react';
import JobStatusIcon from '@/components/job-status';
import JobListItem from '@/components/job-list-item';
// import { useLogs } from '@/stores/log.store';
import React, { useEffect, useState } from 'react';
import { useDatasets } from '@/stores/datasets.store';
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
// import { Editor } from './editor';
import { FormEvent } from 'react';
import { rust } from '@codemirror/lang-rust';
// import { cpp } from '@codemirror/lang-cpp';
import { useMemo, useRef } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { vscodeDark, vscodeLight } from '@uiw/codemirror-theme-vscode';
import { VariantProps } from 'class-variance-authority';
import { register } from 'module';

import Split from 'react-split-grid';

import { useTheme } from 'next-themes';
import './code.css';
import { IDataset } from '@/types/dataset';
import { Label } from '@/components/ui/label';

//
//
//

const extensions = [rust()];

//
//
//
//

interface CompileRes {
  success: boolean;
  message: string;
  error: string;
}

const schema = z.object({
  name: z.string().min(2).max(50),
  language: z.enum(['c', 'rust']),
  sourceCode: z.string(),
  datasetId: z.string().optional(),
  datasetFile: z.any().optional(),
});

export default function Page() {
  const { toast } = useToast();
  const [response, setResponse] = useState<CompileRes>({
    success: false,
    message: '',
    error: '',
  });

  // Load dataset list on page init.
  // const [datasets, setDatasets] = useState<IDataset[]>([]);

  const { datasets, fetchDatasets, uploadDataset } = useDatasets();

  useEffect(() => {
    fetchDatasets();
  }, []);

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

    let datasetId = values.datasetId;
    if (
      values.datasetFile &&
      values.datasetFile instanceof FileList &&
      values.datasetFile.length > 0
    ) {
      const newDataset = await uploadDataset(values.datasetFile[0]);
      form.setValue('datasetId', newDataset.id);
    }

    if (!datasetId) {
      toast({
        title: 'Error',
        description: 'No dataset selected',
      });
      return;
    }

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

    setResponse((await res.json()) as CompileRes);
  };

  const { theme } = useTheme();

  let [columns, setColumns] = useState(`2fr 10px 1fr`);

  const handleDrag = (direction: any, track: any, gridTemplateStyle: any) => {
    setColumns(gridTemplateStyle);
  };

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className="h-full p-4 space-y-4 xl:space-y-0 xl:space-x-4"
      >
        <Split
          gridTemplateColumns={columns}
          onDrag={handleDrag}
          cursor="col-resize"
          minSize={100}
          // @ts-ignore
          render={({ getGridProps, getGutterProps }) => (
            <div className="h-full split-grid" {...getGridProps()}>
              <div className="split-column h-full w-full rounded-lg border bg-card text-card-foreground shadow-sm flex flex-col">
                <FormField
                  control={form.control}
                  name="sourceCode"
                  render={({ field }) => (
                    <FormItem style={{ height: '100%' }}>
                      <FormControl>
                        <CodeMirror
                          {...field}
                          style={{ height: '100%' }}
                          className={'codeEditor'}
                          value={'TEST'}
                          lang="c"
                          height="100%"
                          theme={theme === 'dark' ? vscodeDark : vscodeLight}
                          extensions={extensions}
                          onChange={(value, _) => {
                            field.onChange(value);
                          }}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              </div>

              <div
                {...getGutterProps('column', 1)}
                className="gutter gutter-vertical"
              ></div>

              <div className="split-column h-full flex flex-col space-y-4">
                <Card className="w-full flex flex-col">
                  <CardHeader>
                    <CardTitle>Job Submission</CardTitle>
                  </CardHeader>
                  <CardContent className="flex flex-col space-y-2">
                    <div className="flex justify-stretch flex-wrap space-x-2">
                      <FormField
                        control={form.control}
                        name="name"
                        render={({ field }) => (
                          <FormItem className="grow">
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
                          <FormItem className="grow min-w-[30%]">
                            <FormLabel>Language</FormLabel>
                            <FormControl>
                              <Select
                                onValueChange={(c) => {
                                  console.dir(c);
                                  field.onChange(c);
                                }}
                                defaultValue={'c'}
                              >
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
                    </div>
                    <div className="flex flex-col space-y-2 pt-7 pb-4">
                      <FormField
                        control={form.control}
                        name="datasetId"
                        render={({ field }) => (
                          <FormItem className="grow min-w-[30%]">
                            <FormLabel>Select Existing</FormLabel>
                            <FormControl>
                              <Select
                                onValueChange={(c) => {
                                  console.dir(c);
                                  field.onChange(c);
                                }}
                                defaultValue={datasets[0]?.id}
                              >
                                <SelectTrigger className="w-full">
                                  <SelectValue placeholder="Select an Existing Dataset" />
                                </SelectTrigger>
                                <SelectContent>
                                  <SelectGroup>
                                    {datasets.map((dataset, i) => (
                                      <SelectItem value={dataset.id} key={i}>
                                        {dataset.name}
                                      </SelectItem>
                                    ))}
                                  </SelectGroup>
                                </SelectContent>
                              </Select>
                            </FormControl>
                            <FormMessage />
                          </FormItem>
                        )}
                      />
                      <div className="flex items-center space-x-5 text-xl py-1">
                        <hr className="w-full grow border-gray-300 " />
                        <span>OR</span>
                        <hr className="w-full grow border-gray-300" />
                      </div>

                      <div className="grid w-full items-center gap-1.5">
                        <Label htmlFor="datasetFile">Upload New Dataset</Label>

                        <Input
                          className="w-full"
                          id="datasetFile"
                          type="file"
                          {...form.register('datasetFile')}
                        />
                      </div>
                    </div>
                  </CardContent>

                  <div className="grow" />
                  <Button type="submit" className="rounded-t-none">
                    Submit Job
                  </Button>
                </Card>

                {response.message && response.message != '' && (
                  <Card className="h-full w-full flex flex-col">
                    <div
                      className="h-full w-full px-4 py-3"
                      style={{
                        backgroundColor: 'beige',
                        fontFamily: 'monospace',
                        fontSize: '0.9rem',
                        boxSizing: 'border-box',
                      }}
                    >
                      {response.message}
                    </div>
                  </Card>
                )}
              </div>
            </div>
          )}
        />
      </form>
    </Form>
  );
}

//
// <div className="split-grid" {...getGridProps()}>
//   <div className="split-column">COLUMN A (position 0)</div>
//   <div
//     className="gutter gutter-vertical"
//     {...getGutterProps('column', 1)}
//   />
//   <div className="split-column">COLUMN B (position 2)</div>
//   <div
//     className="gutter gutter-vertical"
//     {...getGutterProps('column', 3)}
//   />
//   <div className="split-column">COLUMN C (position 3)</div>
// </div>
