module Resource exposing
    ( Flags
    , changeToResource
    , getUpdateMessage
    , handleCallback
    , init
    , subscriptions
    , update
    , view
    , viewPinButton
    , viewVersionBody
    , viewVersionHeader
    )

import Callback exposing (Callback(..))
import Colors
import Concourse
import Concourse.BuildStatus
import Concourse.Pagination
    exposing
        ( Page
        , Paginated
        , Pagination
        , chevron
        , chevronContainer
        , equal
        )
import Css
import Date exposing (Date)
import Date.Format
import Dict
import DictView
import Duration exposing (Duration)
import Effects exposing (Effect(..), runEffect, setTitle)
import Erl
import Html.Attributes
import Html.Styled as Html exposing (Html)
import Html.Styled.Attributes
    exposing
        ( attribute
        , class
        , css
        , href
        , id
        , style
        , title
        )
import Html.Styled.Events
    exposing
        ( onClick
        , onMouseEnter
        , onMouseLeave
        , onMouseOut
        , onMouseOver
        )
import Http
import List.Extra
import Maybe.Extra as ME
import NewTopBar.Styles as Styles
import Pinned exposing (ResourcePinState(..), VersionPinState(..))
import QueryString
import Resource.Models as Models exposing (Model)
import Resource.Msgs exposing (Msg(..))
import Resource.Styles
import Routes
import Spinner
import StrictEvents
import Subscription exposing (Subscription(..))
import Time exposing (Time)
import TopBar
import UpdateMsg exposing (UpdateMsg)
import UserState exposing (UserState(..))


type alias Flags =
    { teamName : String
    , pipelineName : String
    , resourceName : String
    , paging : Maybe Concourse.Pagination.Page
    , csrfToken : String
    }


init : Flags -> ( Model, List Effect )
init flags =
    let
        resourceId =
            { teamName = flags.teamName
            , pipelineName = flags.pipelineName
            , resourceName = flags.resourceName
            }

        ( model, effect ) =
            changeToResource flags
                { resourceIdentifier = resourceId
                , pageStatus = Err Models.Empty
                , teamName = flags.teamName
                , pipelineName = flags.pipelineName
                , name = flags.resourceName
                , checkStatus = Models.CheckingSuccessfully
                , checkError = ""
                , checkSetupError = ""
                , hovered = Models.None
                , lastChecked = Nothing
                , pinnedVersion = NotPinned
                , currentPage = Nothing
                , versions =
                    { content = []
                    , pagination =
                        { previousPage = Nothing
                        , nextPage = Nothing
                        }
                    }
                , now = Nothing
                , csrfToken = flags.csrfToken
                , showPinBarTooltip = False
                , pinIconHover = False
                , pinComment = Nothing
                , route =
                    { logical =
                        Routes.Resource
                            flags.teamName
                            flags.pipelineName
                            flags.resourceName
                    , queries = QueryString.empty
                    , page = Nothing
                    , hash = ""
                    }
                , pipeline = Nothing
                , userState = UserStateUnknown
                , userMenuVisible = False
                , pinnedResources = []
                , showPinIconDropDown = False
                }
    in
    ( model
    , [ FetchResource model.resourceIdentifier
      , FetchUser
      , FetchVersionedResources resourceId flags.paging
      ]
    )


changeToResource : Flags -> Model -> ( Model, List Effect )
changeToResource flags model =
    ( { model
        | currentPage = flags.paging
        , versions =
            { content = []
            , pagination =
                { previousPage = Nothing
                , nextPage = Nothing
                }
            }
      }
    , [ FetchVersionedResources model.resourceIdentifier flags.paging ]
    )


updatePinnedVersion : Concourse.Resource -> Model -> Model
updatePinnedVersion resource model =
    case ( resource.pinnedVersion, resource.pinnedInConfig ) of
        ( Nothing, _ ) ->
            case model.pinnedVersion of
                PinningTo _ ->
                    model

                _ ->
                    { model | pinnedVersion = NotPinned }

        ( Just v, True ) ->
            { model | pinnedVersion = PinnedStaticallyTo v }

        ( Just newVersion, False ) ->
            case model.pinnedVersion of
                UnpinningFrom _ ->
                    { model | pinnedVersion = UnpinningFrom newVersion }

                _ ->
                    { model | pinnedVersion = PinnedDynamicallyTo newVersion }


hasPinnedVersion : Model -> Concourse.Version -> Bool
hasPinnedVersion model v =
    case model.pinnedVersion of
        PinnedStaticallyTo pv ->
            v == pv

        PinnedDynamicallyTo pv ->
            v == pv

        UnpinningFrom pv ->
            v == pv

        _ ->
            False


getUpdateMessage : Model -> UpdateMsg
getUpdateMessage model =
    if model.pageStatus == Err Models.NotFound then
        UpdateMsg.NotFound

    else
        UpdateMsg.AOK


handleCallback : Callback -> Model -> ( Model, List Effect )
handleCallback action model =
    case action of
        ResourceFetched (Ok resource) ->
            ( { model
                | pageStatus = Ok ()
                , teamName = resource.teamName
                , pipelineName = resource.pipelineName
                , name = resource.name
                , checkStatus =
                    if resource.failingToCheck then
                        Models.FailingToCheck

                    else
                        Models.CheckingSuccessfully
                , checkError = resource.checkError
                , checkSetupError = resource.checkSetupError
                , lastChecked = resource.lastChecked
                , pinComment = resource.pinComment
              }
                |> updatePinnedVersion resource
            , [ SetTitle <| resource.name ++ " - " ]
            )

        ResourceFetched (Err err) ->
            case Debug.log "failed to fetch resource" err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        ( model, [ RedirectToLogin ] )

                    else if status.code == 404 then
                        ( { model | pageStatus = Err Models.NotFound }, [] )

                    else
                        ( model, [] )

                _ ->
                    ( model, [] )

        VersionedResourcesFetched (Ok ( requestedPage, paginated )) ->
            let
                fetchedPage =
                    permalink paginated.content

                versions =
                    { pagination = paginated.pagination
                    , content =
                        paginated.content
                            |> List.map
                                (\vr ->
                                    let
                                        existingVersion : Maybe Models.Version
                                        existingVersion =
                                            model.versions.content
                                                |> List.Extra.find
                                                    (\v ->
                                                        v.id.versionID == vr.id
                                                    )

                                        enabledStateAccordingToServer : Models.VersionEnabledState
                                        enabledStateAccordingToServer =
                                            if vr.enabled then
                                                Models.Enabled

                                            else
                                                Models.Disabled
                                    in
                                    case existingVersion of
                                        Just ev ->
                                            { ev
                                                | enabled =
                                                    if ev.enabled == Models.Changing then
                                                        Models.Changing

                                                    else
                                                        enabledStateAccordingToServer
                                            }

                                        Nothing ->
                                            { id =
                                                { teamName = model.teamName
                                                , pipelineName =
                                                    model.pipelineName
                                                , resourceName = model.name
                                                , versionID = vr.id
                                                }
                                            , version = vr.version
                                            , metadata = vr.metadata
                                            , enabled = enabledStateAccordingToServer
                                            , expanded = False
                                            , inputTo = []
                                            , outputOf = []
                                            , showTooltip = False
                                            }
                                )
                    }

                newModel =
                    \newPage ->
                        { model
                            | versions = versions
                            , currentPage = newPage
                        }

                chosenModelWith =
                    \requestedPageUnwrapped ->
                        case model.currentPage of
                            Nothing ->
                                newModel <| Just fetchedPage

                            Just page ->
                                if Concourse.Pagination.equal page requestedPageUnwrapped then
                                    newModel <| requestedPage

                                else
                                    model
            in
            case requestedPage of
                Nothing ->
                    ( newModel (Just fetchedPage), [] )

                Just requestedPageUnwrapped ->
                    ( chosenModelWith requestedPageUnwrapped
                    , []
                    )

        VersionedResourcesFetched (Err err) ->
            flip always (Debug.log "failed to fetch versioned resources" err) <|
                ( model, [] )

        InputToFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | inputTo = builds }) model
            , []
            )

        OutputOfFetched (Ok ( versionID, builds )) ->
            ( updateVersion versionID (\v -> { v | outputOf = builds }) model
            , []
            )

        VersionPinned (Ok ()) ->
            let
                newPinnedVersion =
                    Pinned.finishPinning
                        (\pinningTo ->
                            model.versions.content
                                |> List.Extra.find (\v -> v.id == pinningTo)
                                |> Maybe.map .version
                        )
                        model.pinnedVersion
            in
            ( { model | pinnedVersion = newPinnedVersion }, [] )

        VersionPinned (Err _) ->
            ( { model
                | pinnedVersion = NotPinned
              }
            , []
            )

        VersionUnpinned (Ok ()) ->
            ( { model
                | pinnedVersion = NotPinned
              }
            , [ FetchResource model.resourceIdentifier ]
            )

        VersionUnpinned (Err _) ->
            ( { model
                | pinnedVersion = Pinned.quitUnpinning model.pinnedVersion
              }
            , []
            )

        VersionToggled action versionID result ->
            let
                newEnabledState : Models.VersionEnabledState
                newEnabledState =
                    case ( result, action ) of
                        ( Ok (), Models.Enable ) ->
                            Models.Enabled

                        ( Ok (), Models.Disable ) ->
                            Models.Disabled

                        ( Err _, Models.Enable ) ->
                            Models.Disabled

                        ( Err _, Models.Disable ) ->
                            Models.Enabled
            in
            ( updateVersion versionID (\v -> { v | enabled = newEnabledState }) model
            , []
            )

        Checked (Ok ()) ->
            ( { model | checkStatus = Models.CheckingSuccessfully }
            , [ FetchResource model.resourceIdentifier
              , FetchVersionedResources
                    model.resourceIdentifier
                    model.currentPage
              ]
            )

        Checked (Err err) ->
            ( { model | checkStatus = Models.FailingToCheck }
            , case err of
                Http.BadStatus { status } ->
                    if status.code == 401 then
                        [ RedirectToLogin ]

                    else
                        [ FetchResource model.resourceIdentifier ]

                _ ->
                    []
            )

        UserFetched (Ok user) ->
            ( { model | userState = UserStateLoggedIn user }, [] )

        UserFetched (Err _) ->
            ( { model | userState = UserStateLoggedOut }, [] )

        LoggedOut (Ok _) ->
            ( { model
                | userState = UserStateLoggedOut
                , pipeline = Nothing
              }
            , [ NavigateTo "/" ]
            )

        _ ->
            ( model, [] )


update : Msg -> Model -> ( Model, List Effect )
update action model =
    case action of
        AutoupdateTimerTicked timestamp ->
            ( model
            , [ FetchResource model.resourceIdentifier
              , FetchVersionedResources model.resourceIdentifier model.currentPage
              ]
                ++ fetchDataForExpandedVersions model
            )

        LoadPage page ->
            ( { model
                | currentPage = Just page
              }
            , [ FetchVersionedResources model.resourceIdentifier <| Just page
              , NavigateTo <| paginationRoute model.resourceIdentifier page
              ]
            )

        ExpandVersionedResource versionID ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (.id >> (==) versionID)

                newExpandedState : Bool
                newExpandedState =
                    case version of
                        Just v ->
                            not v.expanded

                        Nothing ->
                            False
            in
            ( updateVersion
                versionID
                (\v ->
                    { v | expanded = newExpandedState }
                )
                model
            , if newExpandedState then
                [ FetchInputTo versionID
                , FetchOutputOf versionID
                ]

              else
                []
            )

        ClockTick now ->
            ( { model | now = Just now }, [] )

        NavTo url ->
            ( model, [ NavigateTo url ] )

        TogglePinBarTooltip ->
            ( { model
                | showPinBarTooltip =
                    case model.pinnedVersion of
                        PinnedStaticallyTo _ ->
                            not model.showPinBarTooltip

                        _ ->
                            False
              }
            , []
            )

        ToggleVersionTooltip ->
            let
                pinnedVersionID : Maybe Models.VersionId
                pinnedVersionID =
                    model.versions.content
                        |> List.Extra.find (.version >> hasPinnedVersion model)
                        |> Maybe.map .id

                newModel =
                    case ( model.pinnedVersion, pinnedVersionID ) of
                        ( PinnedStaticallyTo _, Just id ) ->
                            updateVersion
                                id
                                (\v -> { v | showTooltip = not v.showTooltip })
                                model

                        _ ->
                            model
            in
            ( newModel, [] )

        PinVersion versionID ->
            let
                version : Maybe Models.Version
                version =
                    model.versions.content
                        |> List.Extra.find (\v -> v.id == versionID)

                effects : List Effect
                effects =
                    case version of
                        Just v ->
                            [ DoPinVersion
                                versionID
                                model.csrfToken
                            ]

                        Nothing ->
                            []
            in
            ( { model
                | pinnedVersion =
                    Pinned.startPinningTo
                        versionID
                        model.pinnedVersion
              }
            , effects
            )

        UnpinVersion ->
            let
                cmd : Effect
                cmd =
                    DoUnpinVersion
                        model.resourceIdentifier
                        model.csrfToken
            in
            ( { model
                | pinnedVersion = Pinned.startUnpinning model.pinnedVersion
              }
            , [ cmd ]
            )

        ToggleVersion action versionID ->
            ( updateVersion versionID
                (\v ->
                    { v | enabled = Models.Changing }
                )
                model
            , [ DoToggleVersion action
                    versionID
                    model.csrfToken
              ]
            )

        PinIconHover state ->
            ( { model | pinIconHover = state }, [] )

        Hover hovered ->
            ( { model | hovered = hovered }, [] )

        Check ->
            case model.userState of
                UserStateLoggedIn _ ->
                    ( { model | checkStatus = Models.CurrentlyChecking }
                    , [ DoCheck model.resourceIdentifier model.csrfToken ]
                    )

                _ ->
                    ( model, [ RedirectToLogin ] )

        TopBarMsg msg ->
            TopBar.update msg model


updateVersion :
    Models.VersionId
    -> (Models.Version -> Models.Version)
    -> Model
    -> Model
updateVersion versionID updateFunc model =
    let
        newVersionsContent : List Models.Version
        newVersionsContent =
            model.versions.content
                |> List.Extra.updateIf (.id >> (==) versionID) updateFunc

        versions : Paginated Models.Version
        versions =
            model.versions
    in
    { model | versions = { versions | content = newVersionsContent } }


permalink : List Concourse.VersionedResource -> Page
permalink versionedResources =
    case List.head versionedResources of
        Nothing ->
            { direction = Concourse.Pagination.Since 0
            , limit = 100
            }

        Just version ->
            { direction = Concourse.Pagination.From version.id
            , limit = List.length versionedResources
            }


paginationRoute : Concourse.ResourceIdentifier -> Page -> String
paginationRoute rid page =
    let
        ( param, boundary ) =
            case page.direction of
                Concourse.Pagination.Since bound ->
                    ( "since", Basics.toString bound )

                Concourse.Pagination.Until bound ->
                    ( "until", Basics.toString bound )

                Concourse.Pagination.From bound ->
                    ( "from", Basics.toString bound )

                Concourse.Pagination.To bound ->
                    ( "to", Basics.toString bound )

        parsedRoute =
            Erl.parse <|
                "/teams/"
                    ++ rid.teamName
                    ++ "/pipelines/"
                    ++ rid.pipelineName
                    ++ "/resources/"
                    ++ rid.resourceName

        newParsedRoute =
            Erl.addQuery param boundary <| Erl.addQuery "limit" (Basics.toString page.limit) parsedRoute
    in
    Erl.toString newParsedRoute


view : Model -> Html Msg
view model =
    Html.div
        [ style
            [ ( "-webkit-font-smoothing", "antialiased" )
            , ( "font-weight", "700" )
            ]
        ]
        [ Html.map TopBarMsg <| Html.fromUnstyled <| TopBar.view model
        , subpageView model
        , commentBar model
        ]


subpageView : Model -> Html Msg
subpageView model =
    if model.pageStatus == Err Models.Empty then
        Html.div [] []

    else
        Html.div []
            [ header model
            , body model
            ]


header : Model -> Html Msg
header model =
    let
        lastCheckedView =
            case ( model.now, model.lastChecked ) of
                ( Just now, Just date ) ->
                    viewLastChecked now date

                ( _, _ ) ->
                    Html.text ""

        headerHeight =
            60
    in
    Html.div
        [ css
            [ Css.height <| Css.px headerHeight
            , Css.position Css.fixed
            , Css.top <| Css.px Styles.pageHeaderHeight
            , Css.displayFlex
            , Css.alignItems Css.stretch
            , Css.width <| Css.pct 100
            , Css.zIndex <| Css.int 1
            , Css.backgroundColor <| Css.hex "2a2929"
            ]
        ]
        [ Html.h1
            [ css
                [ Css.fontWeight <| Css.int 700
                , Css.marginLeft <| Css.px 18
                , Css.displayFlex
                , Css.alignItems Css.center
                , Css.justifyContent Css.center
                ]
            ]
            [ Html.text model.name ]
        , Html.div
            [ css
                [ Css.displayFlex
                , Css.alignItems Css.center
                , Css.justifyContent Css.center
                , Css.marginLeft (Css.px 24)
                ]
            ]
            [ lastCheckedView ]
        , pinBar model
        , paginationMenu model
        ]


body : Model -> Html Msg
body model =
    let
        headerHeight =
            60
    in
    Html.div
        [ css
            [ Css.padding3
                (Css.px <|
                    headerHeight
                        + Styles.pageHeaderHeight
                        + 10
                )
                (Css.px 10)
                (Css.px 10)
            ]
        , id "body"
        , style
            [ ( "padding-bottom"
              , case model.pinComment of
                    Just _ ->
                        "300px"

                    Nothing ->
                        ""
              )
            ]
        ]
        [ checkSection model
        , viewVersionedResources model
        ]


paginationMenu :
    { a
        | versions : Paginated Models.Version
        , resourceIdentifier : Concourse.ResourceIdentifier
        , hovered : Models.Hoverable
    }
    -> Html Msg
paginationMenu { versions, resourceIdentifier, hovered } =
    let
        previousButtonEventHandler =
            case versions.pagination.previousPage of
                Nothing ->
                    []

                Just pp ->
                    [ onClick <| LoadPage pp ]

        nextButtonEventHandler =
            case versions.pagination.nextPage of
                Nothing ->
                    []

                Just np ->
                    let
                        updatedPage =
                            { np | limit = 100 }
                    in
                    [ onClick <| LoadPage updatedPage ]
    in
    Html.div
        [ id "pagination"
        , style
            [ ( "display", "flex" )
            , ( "align-items", "stretch" )
            ]
        ]
        [ case versions.pagination.previousPage of
            Nothing ->
                Html.div
                    [ style chevronContainer ]
                    [ Html.div
                        [ style <|
                            chevron
                                { direction = "left"
                                , enabled = False
                                , hovered = False
                                }
                        ]
                        []
                    ]

            Just page ->
                Html.div
                    ([ style chevronContainer
                     , onMouseEnter <| Hover Models.PreviousPage
                     , onMouseLeave <| Hover Models.None
                     ]
                        ++ previousButtonEventHandler
                    )
                    [ Html.a
                        [ href <|
                            paginationRoute
                                resourceIdentifier
                                page
                        , attribute "aria-label" "Previous Page"
                        , style <|
                            chevron
                                { direction = "left"
                                , enabled = True
                                , hovered = hovered == Models.PreviousPage
                                }
                        ]
                        []
                    ]
        , case versions.pagination.nextPage of
            Nothing ->
                Html.div
                    [ style chevronContainer ]
                    [ Html.div
                        [ style <|
                            chevron
                                { direction = "right"
                                , enabled = False
                                , hovered = False
                                }
                        ]
                        []
                    ]

            Just page ->
                Html.div
                    ([ style chevronContainer
                     , onMouseEnter <| Hover Models.NextPage
                     , onMouseLeave <| Hover Models.None
                     ]
                        ++ nextButtonEventHandler
                    )
                    [ Html.a
                        [ href <|
                            paginationRoute
                                resourceIdentifier
                                page
                        , attribute "aria-label" "Next Page"
                        , style <|
                            chevron
                                { direction = "right"
                                , enabled = True
                                , hovered = hovered == Models.NextPage
                                }
                        ]
                        []
                    ]
        ]


checkSection :
    { a
        | checkStatus : Models.CheckStatus
        , checkSetupError : String
        , checkError : String
        , hovered : Models.Hoverable
        , userState : UserState
        , teamName : String
    }
    -> Html Msg
checkSection ({ checkStatus, checkSetupError, checkError } as model) =
    let
        failingToCheck =
            checkStatus == Models.FailingToCheck

        checkMessage =
            case checkStatus of
                Models.FailingToCheck ->
                    "checking failed"

                Models.CurrentlyChecking ->
                    "currently checking"

                Models.CheckingSuccessfully ->
                    "checking successfully"

        stepBody =
            if failingToCheck then
                if not (String.isEmpty checkSetupError) then
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkSetupError ]
                        ]
                    ]

                else
                    [ Html.div [ class "step-body" ]
                        [ Html.pre [] [ Html.text checkError ]
                        ]
                    ]

            else
                []

        statusIcon =
            case checkStatus of
                Models.CurrentlyChecking ->
                    Html.fromUnstyled <|
                        Spinner.spinner "14px"
                            [ Html.Attributes.style
                                [ ( "margin", "7px" )
                                ]
                            ]

                _ ->
                    Html.div
                        [ style <|
                            Resource.Styles.checkStatusIcon failingToCheck
                        ]
                        []

        statusBar =
            Html.div
                [ style
                    [ ( "display", "flex" )
                    , ( "justify-content", "space-between" )
                    , ( "align-items", "center" )
                    , ( "flex-grow", "1" )
                    , ( "height", "28px" )
                    , ( "background", Colors.sectionHeader )
                    , ( "padding-left", "5px" )
                    ]
                ]
                [ Html.h3 [] [ Html.text checkMessage ]
                , statusIcon
                ]

        checkBar =
            Html.div
                [ style [ ( "display", "flex" ) ] ]
                [ checkButton model, statusBar ]
    in
    Html.div [ class "resource-check-status" ] <| checkBar :: stepBody


checkButton :
    { a
        | hovered : Models.Hoverable
        , userState : UserState
        , teamName : String
        , checkStatus : Models.CheckStatus
    }
    -> Html Msg
checkButton { hovered, userState, teamName, checkStatus } =
    let
        isHovered =
            hovered == Models.CheckButton

        isCurrentlyChecking =
            checkStatus == Models.CurrentlyChecking

        isUnauthenticated =
            case userState of
                UserStateLoggedIn _ ->
                    False

                _ ->
                    True

        isAuthorized =
            isAuthorizedToTriggerResourceChecks teamName userState

        isClickable =
            (isUnauthenticated || isAuthorized) && not isCurrentlyChecking

        isHighlighted =
            (isClickable && isHovered) || isCurrentlyChecking
    in
    Html.div
        ([ style
            [ ( "height", "28px" )
            , ( "width", "28px" )
            , ( "background-color", Colors.sectionHeader )
            , ( "margin-right", "5px" )
            , ( "cursor"
              , if isClickable then
                    "pointer"

                else
                    "default"
              )
            ]
         , onMouseEnter <| Hover Models.CheckButton
         , onMouseLeave <| Hover Models.None
         ]
            ++ (if isClickable then
                    [ onClick Check ]

                else
                    []
               )
        )
        [ Html.div
            [ style
                [ ( "height", "20px" )
                , ( "width", "20px" )
                , ( "margin", "4px" )
                , ( "background-image"
                  , "url(/public/images/baseline-refresh-24px.svg)"
                  )
                , ( "background-position", "50% 50%" )
                , ( "background-repeat", "no-repeat" )
                , ( "background-size", "contain" )
                , ( "opacity"
                  , if isHighlighted then
                        "1"

                    else
                        "0.5"
                  )
                ]
            ]
            []
        ]


isAuthorizedToTriggerResourceChecks : String -> UserState -> Bool
isAuthorizedToTriggerResourceChecks teamName userState =
    case userState of
        UserStateLoggedIn user ->
            case Dict.get teamName user.teams of
                Just roles ->
                    List.member "member" roles
                        || List.member "owner" roles

                Nothing ->
                    False

        _ ->
            False


commentBar :
    { a
        | pinComment : Maybe String
        , pinnedVersion : Models.PinnedVersion
    }
    -> Html Msg
commentBar { pinComment, pinnedVersion } =
    let
        version =
            case Pinned.stable pinnedVersion of
                Just v ->
                    viewVersion v

                Nothing ->
                    Html.text ""
    in
    case pinComment of
        Nothing ->
            Html.text ""

        Just text ->
            Html.div
                [ id "comment-bar", style Resource.Styles.commentBar ]
                [ Html.div
                    [ style Resource.Styles.commentBarContent ]
                    [ Html.div
                        [ style Resource.Styles.commentBarHeader ]
                        [ Html.div
                            [ style Resource.Styles.commentBarMessageIcon ]
                            []
                        , Html.div
                            [ style Resource.Styles.commentBarPinIcon ]
                            []
                        , version
                        ]
                    , Html.pre [] [ Html.text text ]
                    ]
                ]


pinBar :
    { a
        | pinnedVersion : Models.PinnedVersion
        , showPinBarTooltip : Bool
        , pinIconHover : Bool
    }
    -> Html Msg
pinBar { pinnedVersion, showPinBarTooltip, pinIconHover } =
    let
        pinBarVersion =
            Pinned.stable pinnedVersion

        attrList : List ( Html.Attribute Msg, Bool ) -> List (Html.Attribute Msg)
        attrList =
            List.filter Tuple.second >> List.map Tuple.first

        isPinnedStatically =
            case pinnedVersion of
                PinnedStaticallyTo _ ->
                    True

                _ ->
                    False

        isPinnedDynamically =
            case pinnedVersion of
                PinnedDynamicallyTo _ ->
                    True

                _ ->
                    False
    in
    Html.div
        (attrList
            [ ( id "pin-bar", True )
            , ( style <| Resource.Styles.pinBar { isPinned = ME.isJust pinBarVersion }, True )
            , ( onMouseEnter TogglePinBarTooltip, isPinnedStatically )
            , ( onMouseLeave TogglePinBarTooltip, isPinnedStatically )
            ]
        )
        ([ Html.div
            (attrList
                [ ( id "pin-icon", True )
                , ( style <|
                        Resource.Styles.pinIcon
                            { isPinned = ME.isJust pinBarVersion
                            , isPinnedDynamically = isPinnedDynamically
                            , hover = pinIconHover
                            }
                  , True
                  )
                , ( onClick UnpinVersion, isPinnedDynamically )
                , ( onMouseEnter <| PinIconHover True, isPinnedDynamically )
                , ( onMouseLeave <| PinIconHover False, True )
                ]
            )
            []
         ]
            ++ (case pinBarVersion of
                    Just v ->
                        [ viewVersion v ]

                    _ ->
                        []
               )
            ++ (if showPinBarTooltip then
                    [ Html.div
                        [ id "pin-bar-tooltip"
                        , style Resource.Styles.pinBarTooltip
                        ]
                        [ Html.text "pinned in pipeline config" ]
                    ]

                else
                    []
               )
        )


viewVersionedResources :
    { a
        | versions : Paginated Models.Version
        , pinnedVersion : Models.PinnedVersion
    }
    -> Html Msg
viewVersionedResources { versions, pinnedVersion } =
    versions.content
        |> List.map
            (\v ->
                viewVersionedResource
                    { version = v
                    , pinnedVersion = pinnedVersion
                    }
            )
        |> Html.ul [ class "list list-collapsable list-enableDisable resource-versions" ]


viewVersionedResource :
    { version : Models.Version
    , pinnedVersion : Models.PinnedVersion
    }
    -> Html Msg
viewVersionedResource { version, pinnedVersion } =
    let
        pinState =
            case Pinned.pinState version.version version.id pinnedVersion of
                PinnedStatically _ ->
                    PinnedStatically { showTooltip = version.showTooltip }

                x ->
                    x
    in
    Html.li
        (case ( pinState, version.enabled ) of
            ( Disabled, _ ) ->
                [ style [ ( "opacity", "0.5" ) ] ]

            ( _, Models.Disabled ) ->
                [ style [ ( "opacity", "0.5" ) ] ]

            _ ->
                []
        )
        ([ Html.div
            [ css
                [ Css.displayFlex
                , Css.margin2 (Css.px 5) Css.zero
                ]
            ]
            [ viewEnabledCheckbox
                { enabled = version.enabled
                , id = version.id
                , pinState = pinState
                }
            , viewPinButton
                { versionID = version.id
                , pinState = pinState
                , showTooltip = version.showTooltip
                }
            , viewVersionHeader
                { id = version.id
                , version = version.version
                , pinnedState = pinState
                }
            ]
         ]
            ++ (if version.expanded then
                    [ viewVersionBody
                        { inputTo = version.inputTo
                        , outputOf = version.outputOf
                        , metadata = version.metadata
                        }
                    ]

                else
                    []
               )
        )


viewVersionBody :
    { a
        | inputTo : List Concourse.Build
        , outputOf : List Concourse.Build
        , metadata : Concourse.Metadata
    }
    -> Html Msg
viewVersionBody { inputTo, outputOf, metadata } =
    Html.div
        [ css
            [ Css.displayFlex
            , Css.padding2 (Css.px 5) (Css.px 10)
            ]
        ]
        [ Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "inputs to" ] ]
                , viewBuilds <| listToMap inputTo
                ]
        , Html.div [ class "vri" ] <|
            List.concat
                [ [ Html.div [ css [ Css.lineHeight <| Css.px 25 ] ] [ Html.text "outputs of" ] ]
                , viewBuilds <| listToMap outputOf
                ]
        , Html.div [ class "vri metadata-container" ]
            [ Html.div [ class "list-collapsable-title" ] [ Html.text "metadata" ]
            , viewMetadata metadata
            ]
        ]


viewEnabledCheckbox :
    { a
        | enabled : Models.VersionEnabledState
        , id : Models.VersionId
        , pinState : VersionPinState
    }
    -> Html Msg
viewEnabledCheckbox ({ enabled, id, pinState } as params) =
    let
        clickHandler =
            case enabled of
                Models.Enabled ->
                    [ onClick <| ToggleVersion Models.Disable id ]

                Models.Changing ->
                    []

                Models.Disabled ->
                    [ onClick <| ToggleVersion Models.Enable id ]
    in
    Html.div
        ([ Html.Styled.Attributes.attribute
            "aria-label"
            "Toggle Resource Version Enabled"
         , style <| Resource.Styles.enabledCheckbox params
         ]
            ++ clickHandler
        )
        (case enabled of
            Models.Enabled ->
                []

            Models.Changing ->
                [ Html.fromUnstyled <|
                    Spinner.spinner
                        "12.5px"
                        [ Html.Attributes.style [ ( "margin", "6.25px" ) ] ]
                ]

            Models.Disabled ->
                []
        )


viewPinButton :
    { versionID : Models.VersionId
    , pinState : VersionPinState
    , showTooltip : Bool
    }
    -> Html Msg
viewPinButton { versionID, pinState } =
    let
        eventHandlers =
            case pinState of
                Enabled ->
                    [ onClick <| PinVersion versionID ]

                PinnedDynamically ->
                    [ onClick UnpinVersion ]

                PinnedStatically _ ->
                    [ onMouseOut ToggleVersionTooltip
                    , onMouseOver ToggleVersionTooltip
                    ]

                Disabled ->
                    []

                InTransition ->
                    []
    in
    Html.div
        ([ Html.Styled.Attributes.attribute
            "aria-label"
            "Pin Resource Version"
         , style <| Resource.Styles.pinButton pinState
         ]
            ++ eventHandlers
        )
        (case pinState of
            PinnedStatically { showTooltip } ->
                if showTooltip then
                    [ Html.div
                        [ style
                            [ ( "position", "absolute" )
                            , ( "bottom", "25px" )
                            , ( "background-color", Colors.pinTooltip )
                            , ( "z-index", "2" )
                            , ( "padding", "5px" )
                            , ( "width", "170px" )
                            ]
                        ]
                        [ Html.text "enable via pipeline config" ]
                    ]

                else
                    []

            InTransition ->
                [ Html.fromUnstyled <|
                    Spinner.spinner
                        "12.5px"
                        [ Html.Attributes.style [ ( "margin", "6.25px" ) ] ]
                ]

            _ ->
                []
        )


viewVersionHeader :
    { a
        | id : Models.VersionId
        , version : Concourse.Version
        , pinnedState : VersionPinState
    }
    -> Html Msg
viewVersionHeader { id, version, pinnedState } =
    Html.div
        [ onClick <| ExpandVersionedResource id
        , style <| Resource.Styles.versionHeader pinnedState
        ]
        [ viewVersion version ]


viewVersion : Concourse.Version -> Html Msg
viewVersion version =
    version
        |> Dict.map (always Html.text)
        |> Dict.map (always Html.toUnstyled)
        |> DictView.view
        |> Html.fromUnstyled


viewMetadata : Concourse.Metadata -> Html Msg
viewMetadata metadata =
    Html.dl [ class "build-metadata" ]
        (List.concatMap viewMetadataField metadata)


viewMetadataField : Concourse.MetadataField -> List (Html a)
viewMetadataField field =
    [ Html.dt [] [ Html.text field.name ]
    , Html.dd []
        [ Html.pre [ class "metadata-field" ] [ Html.text field.value ]
        ]
    ]


listToMap : List Concourse.Build -> Dict.Dict String (List Concourse.Build)
listToMap builds =
    let
        insertBuild =
            \build dict ->
                let
                    jobName =
                        case build.job of
                            Nothing ->
                                Debug.crash "Jobless builds shouldn't appear on this page!" ""

                            Just job ->
                                job.jobName

                    oldList =
                        Dict.get jobName dict

                    newList =
                        case oldList of
                            Nothing ->
                                [ build ]

                            Just list ->
                                list ++ [ build ]
                in
                Dict.insert jobName newList dict
    in
    List.foldr insertBuild Dict.empty builds


viewBuilds : Dict.Dict String (List Concourse.Build) -> List (Html Msg)
viewBuilds buildDict =
    List.concatMap (viewBuildsByJob buildDict) <| Dict.keys buildDict


viewLastChecked : Time -> Date -> Html a
viewLastChecked now date =
    let
        ago =
            Duration.between (Date.toTime date) now
    in
    Html.table []
        [ Html.tr
            []
            [ Html.td [] [ Html.text "checked" ]
            , Html.td [ title (Date.Format.format "%b %d %Y %I:%M:%S %p" date) ]
                [ Html.span [] [ Html.text (Duration.format ago ++ " ago") ] ]
            ]
        ]


viewBuildsByJob : Dict.Dict String (List Concourse.Build) -> String -> List (Html Msg)
viewBuildsByJob buildDict jobName =
    let
        oneBuildToLi =
            \build ->
                let
                    link =
                        case build.job of
                            Nothing ->
                                ""

                            Just job ->
                                "/teams/" ++ job.teamName ++ "/pipelines/" ++ job.pipelineName ++ "/jobs/" ++ job.jobName ++ "/builds/" ++ build.name
                in
                Html.li [ class <| Concourse.BuildStatus.show build.status ]
                    [ Html.a
                        [ Html.Styled.Attributes.fromUnstyled <| StrictEvents.onLeftClick <| NavTo link
                        , href link
                        ]
                        [ Html.text <| "#" ++ build.name ]
                    ]
    in
    [ Html.h3 [ class "man pas ansi-bright-black-bg" ] [ Html.text jobName ]
    , Html.ul [ class "builds-list" ]
        (case Dict.get jobName buildDict of
            Nothing ->
                []

            -- never happens
            Just buildList ->
                List.map oneBuildToLi buildList
        )
    ]


fetchDataForExpandedVersions : Model -> List Effect
fetchDataForExpandedVersions model =
    model.versions.content
        |> List.filter .expanded
        |> List.concatMap (\v -> [ FetchInputTo v.id, FetchOutputOf v.id ])


subscriptions : Model -> List (Subscription Msg)
subscriptions model =
    [ OnClockTick (5 * Time.second) AutoupdateTimerTicked
    , OnClockTick Time.second ClockTick
    ]
