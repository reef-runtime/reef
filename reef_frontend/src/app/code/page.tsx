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
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
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
import { cpp } from '@codemirror/lang-cpp';

// import { cpp } from '@codemirror/lang-cpp';
import { useMemo, useRef } from 'react';
import CodeMirror, { Extension } from '@uiw/react-codemirror';
import { vscodeDark, vscodeLight } from '@uiw/codemirror-theme-vscode';
import { VariantProps } from 'class-variance-authority';
import { register } from 'module';

import {
  HoverCard,
  HoverCardContent,
  HoverCardTrigger,
} from '@/components/ui/hover-card';

import Split from 'react-split-grid';

import { useTheme } from 'next-themes';
import './code.css';
import { IDataset } from '@/types/dataset';
import { Label } from '@/components/ui/label';
import { JobLanguage, ITemplate } from '@/types/template';
import { useTemplates } from '@/stores/templates.store';
import IconRustLanguage from '@/components/rust-logo';
import IconCLanguage from '@/components/c-logo';

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

  const { datasets, fetchDatasets, uploadDataset } = useDatasets();

  const { templates, setTemplates, fetchTemplates } = useTemplates();
  const [template, setTemplate] = useState<ITemplate>({
    id: '',
    name: '',
    language: 'c',
    code: '',
    dataset: '',
  });
  const [templateFresh, setTemplateFresh] = useState<boolean>(true);

  // Load datasets and templates on load.
  useEffect(() => {
    fetchDatasets();
    fetchTemplates();
  }, [window.location]);

  useEffect(() => {
    if (templates.length === 0) {
      return;
    }

    setTemplateFresh(true);
    setTemplate(templates[0]);
  }, [templates]);

  const form = useForm<z.infer<typeof schema>>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: '',
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

  const handleDrag = (_direction: any, _track: any, gridTemplateStyle: any) => {
    setColumns(gridTemplateStyle);
  };

  // const [templates, setTemplates] = useState<Template[]>([
  //   {
  //     id: 'c-hello',
  //     name: '[C] Hello World',
  //     code: 'void run(byte * ds, int dsl) {\n\treef_puts("Hello World");\n}',
  //     dataset:
  //       '8b4985d2f8011f74fdf8566611b8cc8cae398ad350bc33f8b5db5bc840f92cbb',
  //     language: 'c',
  //   },
  //   {
  //     id: 'rust-hello',
  //     name: '[Rust] Hello World',
  //     code: 'pub fn run(dataset: &[u8]) -> impl Into<ReefResult> {\n\tlet msg = "Hello World!";\n\treef::reef_log(msg);\n\t/*reef::reef_log(&format!("dataset[43]: {:?}", dataset[43]));*/ println!("Println log 1."); /*if dataset[0] == 13 { panic!("Bad dataset!"); }*/ let mut sum = std::num::Wrapping(0); for val in dataset { sum += val; } println!("sum: {sum}"); "Test Result".to_string() }',
  //     dataset:
  //       '8b4985d2f8011f74fdf8566611b8cc8cae398ad350bc33f8b5db5bc840f92cbb',
  //     language: 'rust',
  //   },
  // ]);

  useEffect(() => {
    if (!templateFresh) {
      console.log('template not fresh');
      return;
    }

    if (!template) {
      console.log('illegal template selection');
      return;
    }

    console.log('USED EFFECT');
    setLanguage(template.language);
    form.setValue('language', template.language);

    setDataset(template.dataset);
    form.setValue('datasetId', template.dataset);

    setSourceCode(template.code);
    form.setValue('sourceCode', template.code);

    form.setValue('name', template.name);

    setTemplateFresh(false);
  }, [template, templateFresh]);

  const [language, setLanguage] = useState<JobLanguage>(template.language);
  const [dataset, setDataset] = useState<string>(template.dataset);
  const [sourceCode, setSourceCode] = useState<string>(template.code);

  //
  // File Size handling
  //

  const UNITS = [
    'byte',
    'kilobyte',
    'megabyte',
    'gigabyte',
    'terabyte',
    'petabyte',
  ];
  const BYTES_PER_KB = 1000;

  function humanFileSize(sizeBytes: number | bigint): string {
    let size = Math.abs(Number(sizeBytes));

    let u = 0;
    while (size >= BYTES_PER_KB && u < UNITS.length - 1) {
      size /= BYTES_PER_KB;
      ++u;
    }

    return new Intl.NumberFormat([], {
      style: 'unit',
      unit: UNITS[u],
      unitDisplay: 'short',
      maximumFractionDigits: 1,
    }).format(size);
  }

  //
  // File sizes
  //

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
              <Card className="split-column h-full w-full rounded-lg border bg-card text-card-foreground shadow-sm flex flex-col">
                <CardHeader className="w-full flex items-center justify-between flex-hor -flex-col px-5 py-2">
                  <div className="flex gap-2 items-end align-top">
                    <FormItem>
                      <FormLabel>Select Template</FormLabel>
                      <FormControl>
                        <Select
                          onValueChange={(newTemplateID) => {
                            console.dir(newTemplateID);
                            const newT = templates.find(
                              (t) => t.id == newTemplateID
                            );
                            if (!newT) {
                              throw `Illegal item: ${newT}`;
                            }
                            setTemplateFresh(true);
                            setTemplate(newT);
                          }}
                          value={template.id}
                        >
                          <SelectTrigger className="w-[20rem]">
                            <SelectValue placeholder="Select a Template" />
                          </SelectTrigger>
                          <SelectContent>
                            <SelectGroup>
                              {templates.map((t, _) => {
                                return (
                                  <SelectItem value={t.id} key={t.id}>
                                    {t.name}
                                  </SelectItem>
                                );
                              })}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>

                    <Button
                      onClick={() => {
                        setTemplateFresh(true);
                      }}
                      type="button"
                      variant="outline"
                    >
                      Apply Template
                    </Button>
                  </div>

                  <div className="flex justify-between align-center">
                    <span className="text-4xl text-muted-foreground self-center mx-[1rem]">
                      {language === 'rust' ? (
                        <IconRustLanguage />
                      ) : (
                        <IconCLanguage />
                      )}
                    </span>

                    <ul className="w-[12rem]">
                      <li className="flex justify-between w-full">
                        <span className="text-sm text-muted-foreground">
                          Lines Of Code
                        </span>
                        <span>{sourceCode.split('\n').length}</span>
                      </li>
                      <li className="flex justify-between w-full">
                        <span className="text-sm text-muted-foreground">
                          Dataset Size
                        </span>
                        {(function () {
                          const sz = datasets.find(
                            (d) => d.id === form.getValues('datasetId')
                          )?.size;

                          if (!sz) {
                            return (
                              <span className="text-sm text-muted-foreground">
                                N/A
                              </span>
                            );
                          }

                          return (
                            <span className="text-sm">{humanFileSize(sz)}</span>
                          );
                        })()}
                      </li>
                    </ul>
                  </div>

                  {/*
                  <Table className="w-[100px] -w-full">
                    <TableHeader>
                      <TableRow>
                        <TableHead className="w-[10px]">LoC</TableHead>
                        <TableHead className="text-right">DS_LEN</TableHead>
                        <TableHead className="text-right">DS_SIZE</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      <TableRow>
                        <TableCell className="font-medium">
                          {sourceCode.split('\n').length}
                        </TableCell>
                        <TableCell className="text-right">
                          {
                            datasets.find(
                              (d) => d.id === form.getValues('datasetId')
                            )?.size
                          }
                        </TableCell>
                        <TableCell className="font-medium text-right">
                          {(function () {
                            const sz = datasets.find(
                              (d) => d.id === form.getValues('datasetId')
                            )?.size;

                            if (!sz) {
                              return 0;
                            }

                            return humanFileSize(sz);
                          })()}
                        </TableCell>
                      </TableRow>
                    </TableBody>
                  </Table>
*/}
                </CardHeader>
                <Separator></Separator>
                <CardContent className="p-0">
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
                            value={sourceCode}
                            lang={language}
                            height="100%"
                            theme={theme === 'dark' ? vscodeDark : vscodeLight}
                            extensions={language === 'c' ? [cpp()] : [rust()]}
                            onChange={(value, _) => {
                              setSourceCode(value);
                              field.onChange(value);
                            }}
                          />
                        </FormControl>
                      </FormItem>
                    )}
                  />
                </CardContent>
              </Card>

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
                                onValueChange={(newLang) => {
                                  console.dir(newLang);
                                  field.onChange(newLang);
                                  setLanguage(newLang as JobLanguage);
                                }}
                                value={language}
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
                                onValueChange={(newDataset) => {
                                  console.dir(newDataset);
                                  field.onChange(newDataset);
                                  setDataset(newDataset);
                                }}
                                value={dataset}
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
                          onChange={(e) => {
                            if (
                              !e.target.files ||
                              e.target.files.length === 0
                            ) {
                              toast({
                                title: 'Not Uploaded',
                                description:
                                  'No file was selected for dataset upload',
                              });
                              return;
                            }

                            const fileCnt = e.target.files.length;

                            for (let i = 0; i < fileCnt; i++) {
                              const file = e.target.files[i];
                              console.log(file);

                              uploadDataset(file).then((newDataset) => {
                                form.setValue('datasetId', newDataset.id);
                                toast({
                                  title: 'File Uploaded Successfully',
                                  description: `Created new dataset '${newDataset.id.substring(0, 16)}...'`,
                                });
                              });
                            }
                          }}
                        />
                      </div>
                    </div>
                  </CardContent>

                  <div className="grow" />
                  <Button type="submit" className="rounded-t-none">
                    Submit Job
                  </Button>
                </Card>

                <Card className="h-full w-full flex flex-col">
                  <div
                    className="h-full w-full px-4 py-3 bg-blue-50 dark:bg-transparent overflow-auto h-full"
                    style={{
                      // backgroundColor: 'beige',
                      fontFamily: 'monospace',
                      fontSize: '0.9rem',
                      boxSizing: 'border-box',
                    }}
                  >
                    <span className="font-bold">
                      {response.message
                        ? response.message.toUpperCase()
                        : 'JOB OUTPUT'}
                    </span>

                    <Separator className="my-5"></Separator>

                    {(function () {
                      if (response.message && response.message != '') {
                        return (
                          <div
                            dangerouslySetInnerHTML={{
                              __html: response.error
                                .replaceAll('\n', '<br>')
                                .replaceAll(' ', '&nbsp;'),
                            }}
                          ></div>
                        );
                      } else {
                        return <div>SUCCESS, show job output here</div>;
                      }
                    })()}
                  </div>
                </Card>
              </div>
            </div>
          )}
        />
      </form>
    </Form>
  );
}
