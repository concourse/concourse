module Routes exposing
    ( Highlight(..)
    , Route(..)
    , SearchType(..)
    , StepID
    , buildRoute
    , dashboardRoute
    , extractPid
    , extractQuery
    , jobRoute
    , parsePath
    , pipelineRoute
    , showHighlight
    , toString
    , tokenToFlyRoute
    )

import Concourse
import Concourse.Pagination as Pagination exposing (Direction(..))
import Url
import Url.Builder as Builder
import Url.Parser
    exposing
        ( (</>)
        , (<?>)
        , Parser
        , custom
        , fragment
        , int
        , map
        , oneOf
        , parse
        , s
        , string
        , top
        )
import Url.Parser.Query as Query


type Route
    = Build { id : Concourse.JobBuildIdentifier, highlight : Highlight }
    | Resource { id : Concourse.ResourceIdentifier, page : Maybe Pagination.Page }
    | Job { id : Concourse.JobIdentifier, page : Maybe Pagination.Page }
    | OneOffBuild { id : Concourse.BuildId, highlight : Highlight }
    | Pipeline { id : Concourse.PipelineIdentifier, groups : List String }
    | Dashboard SearchType
    | FlySuccess (Maybe Int)


type SearchType
    = HighDensity
    | Normal (Maybe String)


type Highlight
    = HighlightNothing
    | HighlightLine StepID Int
    | HighlightRange StepID Int Int


type alias StepID =
    String



-- pages


build : Parser (Route -> a) a
build =
    let
        buildHelper teamName pipelineName jobName buildName h =
            Build
                { id =
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    , buildName = buildName
                    }
                , highlight = h
                }
    in
    map buildHelper
        (s "teams"
            </> string
            </> s "pipelines"
            </> string
            </> s "jobs"
            </> string
            </> s "builds"
            </> string
            </> fragment parseHighlight
        )


oneOffBuild : Parser (Route -> a) a
oneOffBuild =
    map
        (\b h -> OneOffBuild { id = b, highlight = h })
        (s "builds" </> int </> fragment parseHighlight)


parsePage : Maybe Int -> Maybe Int -> Maybe Int -> Maybe Pagination.Page
parsePage since until limit =
    case ( since, until, limit ) of
        ( Nothing, Just u, Just l ) ->
            Just
                { direction = Pagination.Until u
                , limit = l
                }

        ( Just s, Nothing, Just l ) ->
            Just
                { direction = Pagination.Since s
                , limit = l
                }

        _ ->
            Nothing


resource : Parser (Route -> a) a
resource =
    let
        resourceHelper teamName pipelineName resourceName since until limit =
            Resource
                { id =
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , resourceName = resourceName
                    }
                , page = parsePage since until limit
                }
    in
    map resourceHelper
        (s "teams"
            </> string
            </> s "pipelines"
            </> string
            </> s "resources"
            </> string
            <?> Query.int "since"
            <?> Query.int "until"
            <?> Query.int "limit"
        )


job : Parser (Route -> a) a
job =
    let
        jobHelper teamName pipelineName jobName since until limit =
            Job
                { id =
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    }
                , page = parsePage since until limit
                }
    in
    map jobHelper
        (s "teams"
            </> string
            </> s "pipelines"
            </> string
            </> s "jobs"
            </> string
            <?> Query.int "since"
            <?> Query.int "until"
            <?> Query.int "limit"
        )


pipeline : Parser (Route -> a) a
pipeline =
    map
        (\t p g ->
            Pipeline
                { id =
                    { teamName = t
                    , pipelineName = p
                    }
                , groups = g
                }
        )
        (s "teams"
            </> string
            </> s "pipelines"
            </> string
            <?> Query.custom "group" identity
        )


dashboard : Parser (Route -> a) a
dashboard =
    oneOf
        [ map (Normal >> Dashboard) (top <?> Query.string "search")
        , map (Dashboard HighDensity) (s "hd")
        ]


flySuccess : Parser (Route -> a) a
flySuccess =
    map FlySuccess (s "fly_success" <?> Query.int "fly_port")



-- route utils


buildRoute : Concourse.Build -> Route
buildRoute b =
    case b.job of
        Just j ->
            Build
                { id =
                    { teamName = j.teamName
                    , pipelineName = j.pipelineName
                    , jobName = j.jobName
                    , buildName = b.name
                    }
                , highlight = HighlightNothing
                }

        Nothing ->
            OneOffBuild { id = b.id, highlight = HighlightNothing }


jobRoute : Concourse.Job -> Route
jobRoute j =
    Job
        { id =
            { teamName = j.teamName
            , pipelineName = j.pipelineName
            , jobName = j.name
            }
        , page = Nothing
        }


pipelineRoute : { a | name : String, teamName : String } -> Route
pipelineRoute p =
    Pipeline { id = { teamName = p.teamName, pipelineName = p.name }, groups = [] }


dashboardRoute : Bool -> Route
dashboardRoute isHd =
    if isHd then
        Dashboard HighDensity

    else
        Dashboard (Normal Nothing)


showHighlight : Highlight -> String
showHighlight hl =
    case hl of
        HighlightNothing ->
            ""

        HighlightLine id line ->
            "#L" ++ id ++ ":" ++ String.fromInt line

        HighlightRange id line1 line2 ->
            "#L"
                ++ id
                ++ ":"
                ++ String.fromInt line1
                ++ ":"
                ++ String.fromInt line2


parseHighlight : Maybe String -> Highlight
parseHighlight hash =
    case hash of
        Just h ->
            case String.uncons h of
                Just ( 'L', selector ) ->
                    case String.split ":" selector of
                        [ stepID, line1str, line2str ] ->
                            case ( String.toInt line1str, String.toInt line2str ) of
                                ( Just line1, Just line2 ) ->
                                    HighlightRange stepID line1 line2

                                _ ->
                                    HighlightNothing

                        [ stepID, linestr ] ->
                            case String.toInt linestr of
                                Just line ->
                                    HighlightLine stepID line

                                _ ->
                                    HighlightNothing

                        _ ->
                            HighlightNothing

                _ ->
                    HighlightNothing

        _ ->
            HighlightNothing


tokenToFlyRoute : String -> Int -> String
tokenToFlyRoute authToken flyPort =
    Builder.crossOrigin
        ("http://127.0.0.1:" ++ String.fromInt flyPort)
        []
        [ Builder.string "token" authToken ]



-- router


sitemap : Parser (Route -> a) a
sitemap =
    oneOf
        [ resource
        , job
        , dashboard
        , pipeline
        , build
        , oneOffBuild
        , flySuccess
        ]


pageToQueryString : Maybe Pagination.Page -> String
pageToQueryString page =
    case page of
        Nothing ->
            ""

        Just { direction, limit } ->
            "?"
                ++ (case direction of
                        Since id ->
                            "since=" ++ String.fromInt id

                        Until id ->
                            "until=" ++ String.fromInt id

                        From id ->
                            "from=" ++ String.fromInt id

                        To id ->
                            "to=" ++ String.fromInt id
                   )
                ++ "&limit="
                ++ String.fromInt limit


toString : Route -> String
toString route =
    case route of
        Build { id, highlight } ->
            "/teams/"
                ++ id.teamName
                ++ "/pipelines/"
                ++ id.pipelineName
                ++ "/jobs/"
                ++ id.jobName
                ++ "/builds/"
                ++ id.buildName
                ++ showHighlight highlight

        Job { id, page } ->
            "/teams/"
                ++ id.teamName
                ++ "/pipelines/"
                ++ id.pipelineName
                ++ "/jobs/"
                ++ id.jobName
                ++ pageToQueryString page

        Resource { id, page } ->
            "/teams/"
                ++ id.teamName
                ++ "/pipelines/"
                ++ id.pipelineName
                ++ "/resources/"
                ++ id.resourceName
                ++ pageToQueryString page

        OneOffBuild { id, highlight } ->
            "/builds/"
                ++ String.fromInt id
                ++ showHighlight highlight

        Pipeline { id, groups } ->
            "/teams/"
                ++ id.teamName
                ++ "/pipelines/"
                ++ id.pipelineName
                ++ (case groups of
                        [] ->
                            ""

                        gs ->
                            "?group=" ++ String.join "&group=" gs
                   )

        Dashboard (Normal (Just search)) ->
            "/?search=" ++ search

        Dashboard (Normal Nothing) ->
            "/"

        Dashboard HighDensity ->
            "/hd"

        FlySuccess (Just flyPort) ->
            "/fly_success?fly_port=" ++ String.fromInt flyPort

        FlySuccess Nothing ->
            "/fly_success"


parsePath : Url.Url -> Maybe Route
parsePath =
    parse sitemap



-- route utils


extractPid : Route -> Maybe Concourse.PipelineIdentifier
extractPid route =
    case route of
        Build { id } ->
            Just { teamName = id.teamName, pipelineName = id.pipelineName }

        Job { id } ->
            Just { teamName = id.teamName, pipelineName = id.pipelineName }

        Resource { id } ->
            Just { teamName = id.teamName, pipelineName = id.pipelineName }

        Pipeline { id } ->
            Just id

        _ ->
            Nothing


extractQuery : SearchType -> String
extractQuery route =
    case route of
        Normal (Just q) ->
            q

        _ ->
            ""
