#!/usr/bin/env node
var optimist = require('optimist');
var _ = require('under_score');

var argv = optimist
  .string('s').alias('s', 'server')
  .string('t').alias('t', 'tunnel')
  .boolean('r').alias('r', 'reverse')
  .string('a').alias('a', 'allow')
  .string('u').alias('u', 'uuid')
  .string('ssl')
  .string('key')
  .string('cert')
  .argv;

var wsHost = _.last(argv._) || "";

console.log('PARSED server=%s tunnel=%s reverse=%s allow=%s uuid=%s ssl=%s key=%s cert=%s wsHost=%s',
  argv.s || '', argv.t || '', argv.r || '', argv.a || '', argv.u || '', argv.ssl || '', argv.key || '', argv.cert || '', wsHost);
