#lang scheme/base

(provide version)

(define version (vector-ref (current-command-line-arguments) 0))
