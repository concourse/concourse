#lang scheme/base

(provide version)

(define version
  (let ([version-arg-provided (positive? (vector-length (current-command-line-arguments)))])
    (if version-arg-provided
      (vector-ref (current-command-line-arguments) 0)   
      "x.x.x")))
