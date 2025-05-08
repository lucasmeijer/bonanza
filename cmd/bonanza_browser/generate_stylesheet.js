const fs = require('node:fs/promises')
const node = require('@tailwindcss/node')
const process = require('node:process');

async function main() {
  let compiler = await node.compile(`
    @import 'tailwindcss';
    @plugin "daisyui" {
      themes: light --default;
    }

    .bg-neutral {
      background-color: var(--color-neutral);
    }

    .message-contents ul {
      border-left: 1px dotted #555;
      list-style: none;
      padding-inline-start: 25px;
    }
  `, {
    base: process.cwd(),
    onDependency(path) {},
  })
  let compiledCss = compiler.build([
    '[--tab-bg:var(--color-neutral)]',
    'alert-error',
    'alert',
    'bg-amber-100',
    'bg-base-100',
    'bg-base-200',
    'bg-neutral',
    'bg-primary',
    'block',
    'border-base-300',
    'break-all',
    'btn-ghost',
    'btn-outline',
    'btn-primary',
    'btn-square',
    'btn',
    'card-actions',
    'card-body',
    'card-title',
    'card',
    'flex-col',
    'flex',
    'float-right',
    'font-mono',
    'h-auto!',
    'justify-end',
    'inline-block',
    'link-accent',
    'link',
    'max-w-[100rem]',
    'mb-4',
    'message-contents',
    'mt-4',
    'mx-auto',
    'my-2',
    'my-4',
    'navbar',
    'overflow-x-auto',
    'overflow-x-hidden',
    'p-4',
    'shadow-sm',
    'shadow',
    'space-x-4',
    'space-y-4',
    'tab-active',
    'tab-content',
    'tab',
    'table-pin-cols',
    'table',
    'tabs-lift',
    'tabs',
    'text-2xl',
    'text-amber-200',
    'text-fuchsia-300',
    'text-left',
    'text-neutral-content!',
    'text-neutral-content',
    'text-primary-content',
    'text-red-600',
    'text-sm',
    'text-xl',
    'text-xs',
    'textarea',
    'w-1/3',
    'w-2/3',
    'w-full',
    'whitespace-nowrap',
  ])
  let optimizedCss = node.optimize(compiledCss, { minify: true })
  await fs.writeFile(process.argv[2], optimizedCss)
}

main()
