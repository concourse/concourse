module Pipeline.Filter exposing (Suggestion, filterJobs, suggestions)

import Concourse
import Concourse.BuildStatus as BuildStatus exposing (BuildStatus(..))
import List.Extra
import Parser
    exposing
        ( (|.)
        , (|=)
        , Parser
        , Step(..)
        , backtrackable
        , chompUntilEndOr
        , chompWhile
        , end
        , getChompedString
        , getOffset
        , getSource
        , keyword
        , loop
        , map
        , oneOf
        , run
        , spaces
        , succeed
        , symbol
        )
import Simple.Fuzzy


type alias Filter =
    { negate : Bool
    , jobFilter : JobFilter
    }


filterTypes : List String
filterTypes =
    [ "status" ]


type JobFilter
    = Name StringFilter
    | Status StatusFilter


type StringFilter
    = Fuzzy String
    | Exact String
    | StartsWith String


type StatusFilter
    = Paused
    | JobStatus BuildStatus
    | JobRunning
    | IncompleteStatus String


filterJobs : String -> List Concourse.Job -> List Concourse.Job
filterJobs query jobs =
    parseFilters query
        |> List.map Tuple.first
        |> List.foldr runFilter jobs


runFilter : Filter -> List Concourse.Job -> List Concourse.Job
runFilter f =
    let
        negater =
            if f.negate then
                not

            else
                identity
    in
    List.filter (jobMatches f.jobFilter >> negater)


jobMatches : JobFilter -> Concourse.Job -> Bool
jobMatches jf job =
    case jf of
        Name sf ->
            stringMatches sf job.name

        Status sf ->
            statusMatches sf job


statusMatches : StatusFilter -> Concourse.Job -> Bool
statusMatches sf job =
    case sf of
        Paused ->
            job.paused

        JobStatus status ->
            jobBuilds job
                |> List.any (\b -> b.status == status)

        JobRunning ->
            jobBuilds job
                |> List.any (\b -> BuildStatus.isRunning b.status)

        IncompleteStatus _ ->
            False


jobBuilds : Concourse.Job -> List Concourse.Build
jobBuilds job =
    [ job.nextBuild
    , job.finishedBuild
    , job.transitionBuild
    ]
        |> List.filterMap identity


stringMatches : StringFilter -> String -> Bool
stringMatches f =
    case f of
        Fuzzy term ->
            Simple.Fuzzy.match term

        Exact name ->
            (==) name

        StartsWith prefix ->
            String.startsWith prefix


parseFilters : String -> List ( Filter, String )
parseFilters =
    run
        (loop [] <|
            \revFilters ->
                oneOf
                    [ end |> map (\_ -> Done (List.reverse revFilters))
                    , filter |> captureChompedString |> map (\f -> Loop (f :: revFilters))
                    ]
        )
        >> Result.withDefault []


captureChompedString : Parser a -> Parser ( a, String )
captureChompedString parser =
    succeed (\start val end_ source -> ( val, String.slice start end_ source ))
        |= getOffset
        |= parser
        |= getOffset
        |= getSource


filter : Parser Filter
filter =
    succeed Filter
        |. spaces
        |= oneOf
            [ symbol "-" |> map (always True)
            , succeed False
            ]
        |= jobFilter


jobFilter : Parser JobFilter
jobFilter =
    oneOf
        [ backtrackable statusFilter
        , succeed Name |= parseString
        ]


parseString : Parser StringFilter
parseString =
    oneOf
        [ parseQuotedString
        , parseWord |> map Fuzzy
        ]


parseQuotedString : Parser StringFilter
parseQuotedString =
    getChompedString
        (symbol "\""
            |. chompUntilEndOr "\""
            |. oneOf [ symbol "\"", end ]
        )
        |> map
            (\s ->
                if String.endsWith "\"" s && s /= "\"" then
                    Exact <| String.slice 1 -1 s

                else
                    StartsWith <| String.dropLeft 1 s
            )


parseWord : Parser String
parseWord =
    getChompedString
        (chompWhile
            (\c -> c /= ' ' && c /= '\t' && c /= '\n' && c /= '\u{000D}')
        )


statusFilter : Parser JobFilter
statusFilter =
    succeed Status
        |. keyword "status"
        |. symbol ":"
        |. spaces
        |= jobStatus


jobStatus : Parser StatusFilter
jobStatus =
    oneOf
        [ keyword "paused" |> map (\_ -> Paused)
        , keyword "aborted" |> map (\_ -> JobStatus BuildStatusAborted)
        , keyword "errored" |> map (\_ -> JobStatus BuildStatusErrored)
        , keyword "failed" |> map (\_ -> JobStatus BuildStatusFailed)
        , keyword "pending" |> map (\_ -> JobStatus BuildStatusPending)
        , keyword "succeeded" |> map (\_ -> JobStatus BuildStatusSucceeded)
        , keyword "running" |> map (\_ -> JobRunning)
        , parseWord |> map IncompleteStatus
        ]


type alias Suggestion =
    { prev : String
    , cur : String
    }


suggestions : String -> List Suggestion
suggestions query =
    let
        parsedFilters =
            parseFilters query

        ( curFilter, negated ) =
            parsedFilters
                |> List.Extra.last
                |> Maybe.map Tuple.first
                |> Maybe.map (\f -> ( f.jobFilter, f.negate ))
                |> Maybe.withDefault ( Name (Fuzzy ""), False )

        prevFilters =
            parsedFilters
                |> List.map Tuple.second
                |> List.reverse
                |> List.drop 1
                |> List.reverse

        prev =
            if List.isEmpty prevFilters then
                ""

            else
                String.join "" prevFilters ++ " "

        cur =
            case curFilter of
                Name (Fuzzy s) ->
                    filterTypes
                        |> List.filter (String.startsWith s)
                        |> List.map (\v -> v ++ ":")

                Name _ ->
                    []

                Status sf ->
                    case sf of
                        IncompleteStatus status ->
                            [ "paused", "pending", "failed", "errored", "aborted", "running", "succeeded" ]
                                |> List.filter (String.startsWith status)
                                |> List.map (\v -> "status:" ++ v)

                        _ ->
                            []

        prefix =
            if negated then
                List.map (\c -> "-" ++ c)

            else
                identity
    in
    List.map (Suggestion prev) (prefix cur)
