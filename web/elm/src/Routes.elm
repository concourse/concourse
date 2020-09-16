module Routes exposing
    ( DashboardView(..)
    , Highlight(..)
    , Route(..)
    , SearchType(..)
    , StepID
    , Transition
    , buildRoute
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
import Api.Pagination
import Dict
import Maybe.Extra
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
    | Dashboard { searchType : SearchType, dashboardView : DashboardView }
    | FlySuccess Bool (Maybe Int)


type SearchType
    = HighDensity
    | Normal String


type DashboardView
    = ViewNonArchivedPipelines
    | ViewAllPipelines


dashboardViews : List DashboardView
dashboardViews =
    [ ViewNonArchivedPipelines, ViewAllPipelines ]


dashboardViewName : DashboardView -> String
dashboardViewName view =
    case view of
        ViewAllPipelines ->
            "all"

        ViewNonArchivedPipelines ->
            "non_archived"


type Highlight
    = HighlightNothing
    | HighlightLine StepID Int
    | HighlightRange StepID Int Int


type alias StepID =
    String


type alias Transition =
    { from : Route
    , to : Route
    }



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
parsePage from to limit =
    case ( from, to, limit ) of
        ( Nothing, Just t, Just l ) ->
            Just
                { direction = Pagination.To t
                , limit = l
                }

        ( Just f, Nothing, Just l ) ->
            Just
                { direction = Pagination.From f
                , limit = l
                }

        _ ->
            Nothing


resource : Parser (Route -> a) a
resource =
    let
        resourceHelper teamName pipelineName resourceName from to limit =
            Resource
                { id =
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , resourceName = resourceName
                    }
                , page = parsePage from to limit
                }
    in
    map resourceHelper
        (s "teams"
            </> string
            </> s "pipelines"
            </> string
            </> s "resources"
            </> string
            <?> Query.int "from"
            <?> Query.int "to"
            <?> Query.int "limit"
        )


job : Parser (Route -> a) a
job =
    let
        jobHelper teamName pipelineName jobName from to limit =
            Job
                { id =
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    }
                , page = parsePage from to limit
                }
    in
    map jobHelper
        (s "teams"
            </> string
            </> s "pipelines"
            </> string
            </> s "jobs"
            </> string
            <?> Query.int "from"
            <?> Query.int "to"
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
    map (\st view -> Dashboard { searchType = st, dashboardView = view }) <|
        oneOf
            [ (top <?> Query.string "search")
                |> map
                    (Maybe.map (String.replace "+" " ")
                        -- https://github.com/elm/url/issues/32
                        >> Maybe.withDefault ""
                        >> Normal
                    )
            , s "hd" |> map HighDensity
            ]
            <?> dashboardViewQuery


dashboardViewQuery : Query.Parser DashboardView
dashboardViewQuery =
    (Query.enum "view" <|
        Dict.fromList
            (dashboardViews
                |> List.map (\v -> ( dashboardViewName v, v ))
            )
    )
        |> Query.map (Maybe.withDefault ViewNonArchivedPipelines)


flySuccess : Parser (Route -> a) a
flySuccess =
    map (\s -> FlySuccess (s == Just "true"))
        (s "fly_success"
            <?> Query.string "noop"
            <?> Query.int "fly_port"
        )



-- route utils


buildRoute : Int -> String -> Maybe Concourse.JobIdentifier -> Route
buildRoute id name jobId =
    case jobId of
        Just j ->
            Build
                { id =
                    { teamName = j.teamName
                    , pipelineName = j.pipelineName
                    , jobName = j.jobName
                    , buildName = name
                    }
                , highlight = HighlightNothing
                }

        Nothing ->
            OneOffBuild { id = id, highlight = HighlightNothing }


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


toString : Route -> String
toString route =
    case route of
        Build { id, highlight } ->
            Builder.absolute
                [ "teams"
                , id.teamName
                , "pipelines"
                , id.pipelineName
                , "jobs"
                , id.jobName
                , "builds"
                , id.buildName
                ]
                []
                ++ showHighlight highlight

        Job { id, page } ->
            Builder.absolute
                [ "teams"
                , id.teamName
                , "pipelines"
                , id.pipelineName
                , "jobs"
                , id.jobName
                ]
                (Api.Pagination.params page)

        Resource { id, page } ->
            Builder.absolute
                [ "teams"
                , id.teamName
                , "pipelines"
                , id.pipelineName
                , "resources"
                , id.resourceName
                ]
                (Api.Pagination.params page)

        OneOffBuild { id, highlight } ->
            Builder.absolute
                [ "builds"
                , String.fromInt id
                ]
                []
                ++ showHighlight highlight

        Pipeline { id, groups } ->
            Builder.absolute
                [ "teams"
                , id.teamName
                , "pipelines"
                , id.pipelineName
                ]
                (groups |> List.map (Builder.string "group"))

        Dashboard { searchType, dashboardView } ->
            let
                path =
                    case searchType of
                        Normal _ ->
                            []

                        HighDensity ->
                            [ "hd" ]

                queryParams =
                    (case searchType of
                        Normal "" ->
                            []

                        Normal query ->
                            [ Builder.string "search" query ]

                        _ ->
                            []
                    )
                        ++ (case dashboardView of
                                ViewNonArchivedPipelines ->
                                    []

                                _ ->
                                    [ Builder.string "view" <| dashboardViewName dashboardView ]
                           )
            in
            Builder.absolute path queryParams

        FlySuccess noop flyPort ->
            Builder.absolute [ "fly_success" ] <|
                (flyPort
                    |> Maybe.map (Builder.int "fly_port")
                    |> Maybe.Extra.toList
                )
                    ++ (if noop then
                            [ Builder.string "noop" "true" ]

                        else
                            []
                       )


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
        Normal q ->
            q

        _ ->
            ""
