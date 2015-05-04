#lang scheme/base

(require (only-in scribble/manual
                  make-box-splice vertical-inset-style boxed-style
                  add-background-label hspace
                  with-togetherable-racket-variables racketblock0)
         (only-in scribble/base tt)
         (only-in scribble/tag make-section-tag)
         (only-in scribble/core
                  block? block-width make-toc-target2-element
                  make-index-element table?  make-style make-table-columns
                  make-part-relative-element collect-info-parents part-tags
                  make-part plain)
         (only-in scribble/racket to-element)
         (only-in scribble/struct
                  make-blockquote make-table make-omitable-paragraph make-flow
                  flow-paragraphs paragraph? paragraph-content)
         (only-in scribble/manual-struct make-thing-index-desc)
         (only-in scribble/private/manual-defaults post-process) ; todon't
         (only-in racket/list append* append-map)
         (only-in racket/string string-join)
         (for-syntax racket/base syntax/parse))

(provide defthing defthing*)

(begin-for-syntax
 (define-splicing-syntax-class kind-kw
   #:description "#:kind keyword"
   (pattern (~optional (~seq #:kind kind)
                       #:defaults ([kind #'#f]))))

 (define-splicing-syntax-class value-kw
   #:description "#:value keyword"
   (pattern (~optional (~seq #:value value)
                       #:defaults ([value #'no-value])))))

(define-syntax (defthing stx)
  (syntax-parse stx
    [(_ kind:kind-kw 
        (~optional (~seq #:id id-expr)
                   #:defaults ([id-expr #'#f]))
        id 
        result 
        value:value-kw
        desc ...)
     #'(with-togetherable-racket-variables
        ()
        ()
        (*defthing kind.kind
                   (list (or id-expr (quote-syntax id)))
                   (list 'id)
                   (list (racketblock0 result))
                   (lambda () (list desc ...))))]))

(define chain-of-things (make-parameter '()))

(define-syntax (defthing* stx)
  (syntax-parse stx
    [(_ kind:kind-kw ([id result value:value-kw] ...) desc ...)
     #'(with-togetherable-racket-variables
        ()
        ()
        (*defthing kind.kind
                   (list (quote-syntax id) ...)
                   (list 'id ...)
                   (list (racketblock0 result) ...)
                   (lambda () (list desc ...))))]))

(define (*defthing kind stx-ids names result-contracts content-thunk)
  (make-box-splice
    (cons
      (make-blockquote
        vertical-inset-style
        (list
          (make-table
            boxed-style
            (append*
              (for/list ([stx-id (in-list stx-ids)]
                         [name (in-list names)]
                         [result-contract (in-list result-contracts)]
                         [i (in-naturals)])
                (thing-header kind stx-id name result-contract i))))))
      (parameterize ([chain-of-things (append names (chain-of-things))])
        (content-thunk)))))

(define (thing-header kind stx-id name result-contract i)
  (let* ([thing-name
           (string-join
             (map symbol->string (append (chain-of-things) (list name)))
             ".")]
         [contract-block
           (if (block? result-contract)
             result-contract
             (make-omitable-paragraph (list result-contract)))]
         [thing-id
           (make-part-relative-element
             (lambda (ci)
               (define parent-tag (car (part-tags (car (collect-info-parents ci)))))
               (let ([s (tt (symbol->string name))]
                     [qs (tt thing-name)]
                     [tag (list 'def (format "~a.~a" parent-tag thing-name))])
                 (make-toc-target2-element
                   #f
                   (make-index-element
                     #f
                     (to-element #:defn? #t s)
                     tag
                     (list (datum-intern-literal thing-name))
                     (list (to-element qs))
                     null)
                   tag
                   (to-element #:defn? #t qs))))
             (lambda () (symbol->string name))
             (lambda () (symbol->string name)))])
    (list
      (list
        ((if (zero? i) (add-background-label (or kind "value")) values)
         (top-align
           make-table-if-necessary
           "argcontract"
           (list
             (append
               (list (list (make-omitable-paragraph
                             (list thing-id))))
               (list
                 (to-flow (list spacer ":" spacer))
                 (list contract-block))))))))))

(define top-align-styles (make-hash))
(define (top-align make-table style-name cols)
  (if (null? cols)
    (make-table style-name null)
    (let* ([n (length (car cols))]
           [k (cons style-name n)])
      (make-table
        (hash-ref top-align-styles
                  k
                  (lambda ()
                    (define s
                      (make-style style-name
                                  (list (make-table-columns (for/list ([i n])
                                                              (make-style #f '(top)))))))
                    (hash-set! top-align-styles k s)
                    s))
        cols))))

(define spacer (hspace 1))

(define (to-flow e)
  (make-flow (list (make-omitable-paragraph (list e)))))

(define (make-table-if-necessary style content)
  (if (= 1 (length content))
    (let ([paras (append-map flow-paragraphs (car content))])
      (if (andmap paragraph? paras)
        (list (make-omitable-paragraph (append-map paragraph-content paras)))
        (list (make-table style content))))
    (list (make-table style content))))
