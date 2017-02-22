module TeamSelection exposing (Model, Msg, init, update, view, subscriptions)

import Erl
import Html exposing (Html)
import Html.Attributes as Attributes exposing (id, class)
import Html.Events as Events
import Http
import Navigation
import String
import Task
import Concourse
import Concourse.Team
import StrictEvents exposing (onLeftClick)


type alias Ports =
    { title : String -> Cmd Msg
    }


type alias Model =
    { teamFilter : String
    , teams : Maybe (List Concourse.Team)
    , redirect : String
    }


type Msg
    = Noop
    | FilterTeams String
    | TeamsFetched (Result Http.Error (List Concourse.Team))
    | SelectTeam String


init : Ports -> String -> ( Model, Cmd Msg )
init ports redirect =
    ( { teamFilter = ""
      , teams = Nothing
      , redirect = redirect
      }
    , Cmd.batch
        [ Task.attempt TeamsFetched Concourse.Team.fetchTeams
        , ports.title "Login - "
        ]
    )


update : Msg -> Model -> ( Model, Cmd Msg )
update action model =
    case action of
        Noop ->
            ( model, Cmd.none )

        FilterTeams newTeamFilter ->
            ( { model | teamFilter = newTeamFilter }
            , Cmd.none
            )

        TeamsFetched (Ok teams) ->
            ( { model | teams = Just teams }
            , Cmd.none
            )

        TeamsFetched (Err err) ->
            flip always (Debug.log ("failed to fetch teams") (err)) <|
                ( model, Cmd.none )

        SelectTeam teamName ->
            ( model
            , Navigation.newUrl <| loginRoute model.redirect teamName
            )


loginRoute : String -> String -> String
loginRoute redirect teamName =
    routeMaybeRedirect redirect <| "/teams/" ++ teamName ++ "/login"


routeMaybeRedirect : String -> String -> String
routeMaybeRedirect redirect route =
    if redirect /= "" then
        let
            parsedRoute =
                Erl.parse route
        in
            let
                newParsedRoute =
                    Erl.addQuery "redirect" redirect parsedRoute
            in
                Erl.toString newParsedRoute
    else
        route


indexPageUrl : String
indexPageUrl =
    "/"


view : Model -> Html Msg
view model =
    let
        filteredTeams =
            filterTeams model.teamFilter <| Maybe.withDefault [] model.teams
    in
        Html.div [ class "login-page" ]
            [ Html.div
                [ class "small-title" ]
                [ Html.text "select a team to login" ]
            , Html.div
                [ class "login-box team-selection" ]
                [ Html.form
                    [ Events.onSubmit <|
                        case ( List.head filteredTeams, model.teamFilter ) of
                            ( Nothing, _ ) ->
                                Noop

                            ( Just _, "" ) ->
                                Noop

                            ( Just firstTeam, _ ) ->
                                SelectTeam firstTeam.name
                    , class "filter-form input-holder"
                    ]
                    [ Html.i [ class "fa fa-fw fa-search search-icon" ] []
                    , Html.input
                        [ class "search-input"
                        , Attributes.placeholder "filter teams"
                        , Attributes.autofocus True
                        , Attributes.required True
                        , Events.onInput FilterTeams
                        ]
                        []
                    , Html.button
                        [ class "clear-button"
                        , Attributes.type_ "reset"
                        , Attributes.tabindex -1
                        , Events.onClick (FilterTeams "")
                        ]
                        [ Html.i [ class "fa fa-fw fa-times-circle" ] [] ]
                    ]
                , case model.teams of
                    Nothing ->
                        viewLoading

                    Just _ ->
                        Html.div [ class "teams-list" ] <|
                            List.map (viewTeam model.redirect) filteredTeams
                ]
            ]


viewLoading : Html action
viewLoading =
    Html.div [ class "loading" ]
        [ Html.i [ class "fa fa-fw fa-spin fa-circle-o-notch" ] []
        ]


viewTeam : String -> Concourse.Team -> Html Msg
viewTeam redirect team =
    Html.a
        [ onLeftClick (SelectTeam team.name)
        , Attributes.href <| loginRoute redirect team.name
        ]
        [ Html.text <| team.name ]


filterTeams : String -> List Concourse.Team -> List Concourse.Team
filterTeams teamFilter teams =
    let
        filteredList =
            List.filter
                (teamNameContains <| String.toLower teamFilter)
                teams
    in
        let
            ( startingTeams, notStartingTeams ) =
                List.partition (teamNameStartsWith <| String.toLower teamFilter) filteredList
        in
            let
                ( caseSensitive, notCaseSensitive ) =
                    List.partition (teamNameStartsWithSensitive teamFilter) startingTeams
            in
                caseSensitive ++ notCaseSensitive ++ notStartingTeams


teamNameContains : String -> Concourse.Team -> Bool
teamNameContains substring team =
    String.contains substring <|
        String.toLower team.name


teamNameStartsWith : String -> Concourse.Team -> Bool
teamNameStartsWith substring team =
    String.startsWith substring <|
        String.toLower team.name


teamNameStartsWithSensitive : String -> Concourse.Team -> Bool
teamNameStartsWithSensitive substring team =
    String.startsWith substring team.name


subscriptions : Model -> Sub Msg
subscriptions model =
    Sub.none
