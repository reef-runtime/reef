'use client';

import { useEffect } from 'react';
import { useToast } from '@/components/ui/use-toast';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';

import init, {
  get_connect_path,
  init_node,
  run_node,
  parse_websocket_data,
} from '@/lib/node_web_generated/reef_node_web';

export default function Page() {
  const { toast } = useToast();

  useEffect(() => {
    console.log('INIT');
    run();
  }, []);

  return (
    <main className="p-4 h-full w-full">
      <Card className="flex flex-col h-full w-full">
        <CardHeader>
          <CardTitle>Node Web</CardTitle>
        </CardHeader>
        <CardContent className="h-full overflow-hidden">Node web</CardContent>
      </Card>
    </main>
  );
}

async function run() {
  await init();

  let connect_path = get_connect_path();
  console.log(connect_path);
}
