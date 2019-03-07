module Routes exposing
    ( Highlight(..)
    , Route(..)
    , SearchType(..)
    , StepID
    , buildRoute
    , dashboardRoute
    , extractPid
    , jobRoute
    , parsePath
    , pipelineRoute
    , showHighlight
    , toString
    , tokenToFlyRoute
    )

import Concourse
import Concourse.Pagination as Pagination exposing (Direction(..))
import Navigation exposing (Location)
import QueryString
import UrlParser
    exposing
        ( (</>)
        , (<?>)
        , Parser
        , custom
        , int
        , intParam
        , map
        , oneOf
        , s
        , string
        , stringParam
        )


type Route
    = Build { id : Concourse.JobBuildIdentifier, highlight : Highlight }
    | Resource { id : Concourse.ResourceIdentifier, page : Maybe Pagination.Page }
    | Job { id : Concourse.JobIdentifier, page : Maybe Pagination.Page }
    | OneOffBuild { id : Concourse.BuildId, highlight : Highlight }
    | Pipeline { id : Concourse.PipelineIdentifier, groups : List String }
    | Dashboard SearchType
    | FlySuccess { flyPort : Maybe Int }


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


build : Parser ((Highlight -> Route) -> a) a
build =
    let
        buildHelper teamName pipelineName jobName buildName highlight =
            Build
                { id =
                    { teamName = teamName
                    , pipelineName = pipelineName
                    , jobName = jobName
                    , buildName = buildName
                    }
                , highlight = highlight
                }
    in
    map buildHelper (s "teams" </> string </> s "pipelines" </> string </> s "jobs" </> string </> s "builds" </> string)


oneOffBuild : Parser ((Highlight -> Route) -> a) a
oneOffBuild =
    map (\b h -> OneOffBuild { id = b, highlight = h }) (s "builds" </> int)


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
            <?> intParam "since"
            <?> intParam "until"
            <?> intParam "limit"
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
            <?> intParam "since"
            <?> intParam "until"
            <?> intParam "limit"
        )


pipeline : Parser ((List String -> Route) -> a) a
pipeline =
    map (\t p g -> Pipeline { id = { teamName = t, pipelineName = p }, groups = g })
        (s "teams" </> string </> s "pipelines" </> string)


dashboard : Parser (Route -> a) a
dashboard =
    oneOf
        [ map (Normal >> Dashboard) (s "" <?> stringParam "search")
        , map (Dashboard HighDensity) (s "hd")
        ]


flySuccess : Parser (Route -> a) a
flySuccess =
    map (\p -> FlySuccess { flyPort = p }) (s "fly_success" <?> intParam "fly_port")



-- route utils


buildRoute : Concourse.Build -> Route
buildRoute build =
    case build.job of
        Just j ->
            Build
                { id =
                    { teamName = j.teamName
                    , pipelineName = j.pipelineName
                    , jobName = j.jobName
                    , buildName = build.name
                    }
                , highlight = HighlightNothing
                }

        Nothing ->
            OneOffBuild { id = build.id, highlight = HighlightNothing }


jobRoute : Concourse.Job -> Route
jobRoute j =
    Job { id = { teamName = j.teamName, pipelineName = j.pipelineName, jobName = j.name }, page = Nothing }


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
            "#L" ++ id ++ ":" ++ Basics.toString line

        HighlightRange id line1 line2 ->
            "#L"
                ++ id
                ++ ":"
                ++ Basics.toString line1
                ++ ":"
                ++ Basics.toString line2


parseHighlight : String -> Highlight
parseHighlight hash =
    case String.uncons hash of
        Just ( 'L', selector ) ->
            case String.split ":" selector of
                [ stepID, line1str, line2str ] ->
                    case ( String.toInt line1str, String.toInt line2str ) of
                        ( Ok line1, Ok line2 ) ->
                            HighlightRange stepID line1 line2

                        _ ->
                            HighlightNothing

                [ stepID, linestr ] ->
                    case String.toInt linestr of
                        Ok line ->
                            HighlightLine stepID line

                        _ ->
                            HighlightNothing

                _ ->
                    HighlightNothing

        _ ->
            HighlightNothing


tokenToFlyRoute : String -> Int -> String
tokenToFlyRoute authToken flyPort =
    let
        queryString =
            QueryString.empty
                |> QueryString.add "token" authToken
                |> QueryString.render
    in
    "http://127.0.0.1:" ++ Basics.toString flyPort ++ queryString



-- router


sitemap : Parser (Route -> a) a
sitemap =
    oneOf
        [ resource
        , job
        , dashboard
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
                            "since=" ++ Basics.toString id

                        Until id ->
                            "until=" ++ Basics.toString id

                        From id ->
                            "from=" ++ Basics.toString id

                        To id ->
                            "to=" ++ Basics.toString id
                   )
                ++ "&limit="
                ++ Basics.toString limit


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
                ++ Basics.toString id
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
                            "?groups=" ++ String.join "&groups=" gs
                   )

        Dashboard (Normal (Just search)) ->
            "/?search=" ++ search

        Dashboard (Normal Nothing) ->
            "/"

        Dashboard HighDensity ->
            "/hd"

        FlySuccess { flyPort } ->
            "/fly_success"
                ++ (case flyPort of
                        Nothing ->
                            ""

                        Just fp ->
                            "?fly_port=" ++ Basics.toString fp
                   )


highlight : Parser (Highlight -> a) a
highlight =
    custom "HIGHLIGHT" <|
        parseHighlight
            >> Ok


parsePath : Location -> Route
parsePath location =
    case
        ( UrlParser.parsePath sitemap location
        , UrlParser.parsePath pipeline location
        , UrlParser.parsePath build location
        , UrlParser.parsePath oneOffBuild location
        )
    of
        ( Just route, _, _, _ ) ->
            route

        ( _, Just f, _, _ ) ->
            QueryString.parse location.search
                |> QueryString.all "groups"
                |> f

        ( _, _, Just f, _ ) ->
            UrlParser.parseHash highlight location
                |> Maybe.withDefault HighlightNothing
                |> f

        ( _, _, _, Just f ) ->
            UrlParser.parseHash highlight location
                |> Maybe.withDefault HighlightNothing
                |> f

        _ ->
            Dashboard (Normal Nothing)



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

        OneOffBuild _ ->
            Nothing

        Dashboard _ ->
            Nothing

        FlySuccess _ ->
            Nothing
