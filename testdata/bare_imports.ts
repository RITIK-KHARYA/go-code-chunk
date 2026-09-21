import dflt from './default'
import { helper, format as fmt } from './utils'
import * as utils from './utils'

function run(): string {
  const a = helper(1)
  const b = fmt(a)
  const c = utils.helper(2)
  const d = dflt()
  return [a, b, c, d].join(',')
}