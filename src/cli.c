#include <stdlib.h>
#include <getopt.h>
#include <string.h>
#include <unistd.h>
#include <libgen.h>

#include "../include/tanabata.h"
#include "../include/cli.h"

// Stylization macros
#define TABLE_HEADER(s) "[7;36m"s"[0m"
#define HIGHLIGHT(s)    "[0;36m"s"[0m"
#define ERROR(s)        "[0;31m"s"[0m"

static Tanabata tanabata;

// Print the list of all sasa
void print_sasa_all() {
    printf(TABLE_HEADER("Sasa ID\tCreation timestamp\tFile path\n"));
    for (uint64_t i = 0; i < tanabata.sasahyou.size; i++) {
        if (tanabata.sasahyou.database[i].id != HOLE_ID) {
            printf("%7lx\t%18lu\t%s\n",
                   tanabata.sasahyou.database[i].id, tanabata.sasahyou.database[i].created_ts,
                   tanabata.sasahyou.database[i].path);
        }
    }
}

// Print the list of all tanzaku
void print_tanzaku_all() {
    printf(TABLE_HEADER("Tanzaku ID\tCreation timestamp\tName\n"));
    for (uint64_t i = 0; i < tanabata.sappyou.size; i++) {
        if (tanabata.sappyou.database[i].id != HOLE_ID) {
            printf("%10lu\t%18lu\t%s\n",
                   tanabata.sappyou.database[i].id, tanabata.sappyou.database[i].created_ts,
                   tanabata.sappyou.database[i].name);
        }
    }
}

// Sasa view menu handler
int menu_view_sasa() {
    char input[16];
    printf(HIGHLIGHT("Enter sasa ID:     "));
    fgets(input, 16, stdin);
    char *endptr;
    uint64_t sasa_id = strtoull(input, &endptr, 16);
    if (*input != '\n' && *endptr == '\n') {
        Sasa current_sasa = tanabata_sasa_get_by_id(&tanabata, sasa_id);
        if (current_sasa.id != HOLE_ID) {
            printf(HIGHLIGHT("File path")"          %s\n"
                   HIGHLIGHT("Added timestamp")"    %lu\n",
                   current_sasa.path, current_sasa.created_ts);
            Tanzaku *related_tanzaku = tanabata_tanzaku_get_by_sasa(&tanabata, current_sasa.id);
            if (related_tanzaku != NULL) {
                printf(HIGHLIGHT("\n↓ Related tanzaku ↓\n"));
                for (Tanzaku *current_tanzaku = related_tanzaku;
                     current_tanzaku->id != HOLE_ID; current_tanzaku++) {
                    printf("'%s'\n", current_tanzaku->name);
                }
                printf(HIGHLIGHT("↑ Related tanzaku ↑\n"));
            } else {
                printf(HIGHLIGHT("\nNo related tanzaku\n"));
            }
            return 0;
        }
        fprintf(stderr, ERROR("No sasa with this ID\n"));
        return 1;
    }
    fprintf(stderr, ERROR("Invalid ID\n"));
    return 1;
}

// Tanzaku view menu handler
int menu_view_tanzaku() {
    char input[16];
    printf(HIGHLIGHT("Enter tanzaku ID:  "));
    fgets(input, 16, stdin);
    char *endptr;
    uint64_t tanzaku_id = strtoull(input, &endptr, 16);
    if (*input != '\n' && *endptr == '\n') {
        Tanzaku current_tanzaku = tanabata_tanzaku_get_by_id(&tanabata, tanzaku_id);
        if (current_tanzaku.id != HOLE_ID) {
            printf(HIGHLIGHT("Name")"               %s\n"
                   HIGHLIGHT("Created timestamp")"  %lu\n"
                   HIGHLIGHT("\n↓ Description ↓\n")
                   "%s\n"
                   HIGHLIGHT("↑ Description ↑\n"),
                   current_tanzaku.name, current_tanzaku.created_ts, current_tanzaku.description);
            Sasa *related_sasa = tanabata_sasa_get_by_tanzaku(&tanabata, tanzaku_id);
            if (related_sasa != NULL) {
                printf(HIGHLIGHT("\n↓ Related sasa ↓\n"));
                for (Sasa *current_sasa = related_sasa;
                     current_sasa->id != HOLE_ID; current_sasa++) {
                    printf("'%s'\n", current_sasa->path);
                }
                printf(HIGHLIGHT("↑ Related sasa ↑\n"));
            } else {
                printf(HIGHLIGHT("\nNo related sasa\n"));
            }
            return 0;
        }
        fprintf(stderr, ERROR("No tanzaku with this ID\n"));
        return 1;
    }
    fprintf(stderr, ERROR("Invalid ID\n"));
    return 1;
}

// Sasa add menu handler
int menu_add_sasa() {
    char path[4096];
    printf(HIGHLIGHT("Enter file path: "));
    fgets(path, 4096, stdin);
    if (*path != '\n') {
        path[strlen(path) - 1] = 0;
        if (tanabata_sasa_add(&tanabata, path) == 0) {
            if (tanabata_save(&tanabata) == 0) {
                printf("Successfully added file to database\n");
                return 0;
            }
        }
        fprintf(stderr, ERROR("Failed to add file to database\n"));
        return 1;
    }
    return 1;
}

// Tanzaku add menu handler
int menu_add_tanzaku() {
    char name[4096];
    char description[4096];
    printf(HIGHLIGHT("Enter tanzaku name:        "));
    fgets(name, 4096, stdin);
    printf(HIGHLIGHT("Enter tanzaku description: "));
    fgets(description, 4096, stdin);
    if (*name != '\n') {
        name[strlen(name) - 1] = 0;
        description[strlen(description) - 1] = 0;
        if (tanabata_tanzaku_add(&tanabata, name, description) == 0) {
            if (tanabata_save(&tanabata) == 0) {
                printf("Successfully added tanzaku to database\n");
                return 0;
            }
        }
        fprintf(stderr, ERROR("Failed to add tanzaku to database\n"));
        return 1;
    }
    return 1;
}

// Kazari add menu handler
int menu_add_kazari() {
    char input[16];
    printf(HIGHLIGHT("Enter sasa ID:    "));
    fgets(input, 16, stdin);
    char *endptr;
    uint64_t sasa_id = strtoull(input, &endptr, 16);
    if (*input == '\n' || *endptr != '\n') {
        fprintf(stderr, ERROR("Invalid ID\n"));
        return 1;
    }
    printf(HIGHLIGHT("Enter tanzaku ID: "));
    fgets(input, 16, stdin);
    uint64_t tanzaku_id = strtoull(input, &endptr, 16);
    if (*input == '\n' || *endptr != '\n') {
        fprintf(stderr, ERROR("Invalid ID\n"));
        return 1;
    }
    if (tanabata_kazari_add(&tanabata, sasa_id, tanzaku_id) == 0) {
        printf("Successfully added kazari\n");
        return 0;
    }
    fprintf(stderr, ERROR("Failed to add kazari\n"));
    return 1;
}

// Sasa remove menu handler
int menu_rem_sasa() {
    char input[16];
    printf(HIGHLIGHT("Enter sasa ID: "));
    fgets(input, 16, stdin);
    char *endptr;
    uint64_t sasa_id = strtoull(input, &endptr, 16);
    if (*input != '\n' && *endptr == '\n') {
        if (tanabata_sasa_rem_by_id(&tanabata, sasa_id) == 0) {
            if (tanabata_save(&tanabata) == 0) {
                printf("Successfully removed sasa\n");
                return 0;
            }
        }
        fprintf(stderr, ERROR("Failed to remove sasa\n"));
        return 1;
    }
    fprintf(stderr, "Invalid ID\n");
    return 1;
}

// Tanzaku remove menu handler
int menu_rem_tanzaku() {
    char input[16];
    printf(HIGHLIGHT("Enter tanzaku ID: "));
    fgets(input, 16, stdin);
    char *endptr;
    uint64_t tanzaku_id = strtoull(input, &endptr, 16);
    if (*input != '\n' && *endptr == '\n') {
        if (tanabata_tanzaku_rem_by_id(&tanabata, tanzaku_id) == 0) {
            if (tanabata_save(&tanabata) == 0) {
                printf("Successfully removed tanzaku\n");
                return 0;
            }
        }
        fprintf(stderr, ERROR("Failed to remove tanzaku\n"));
        return 1;
    }
    fprintf(stderr, ERROR("Invalid ID\n"));
    return 1;
}

// Kazari remove menu handler
int menu_rem_kazari() {
    char input[16];
    printf(HIGHLIGHT("Enter sasa ID:    "));
    fgets(input, 16, stdin);
    char *endptr;
    uint64_t sasa_id = strtoull(input, &endptr, 16);
    if (*input == '\n' || *endptr != '\n') {
        fprintf(stderr, "Invalid ID\n");
        return 1;
    }
    printf(HIGHLIGHT("Enter tanzaku ID: "));
    fgets(input, 16, stdin);
    uint64_t tanzaku_id = strtoull(input, &endptr, 16);
    if (*input == '\n' || *endptr != '\n') {
        fprintf(stderr, "Invalid ID\n");
        return 1;
    }
    if (tanabata_kazari_rem(&tanabata, sasa_id, tanzaku_id) == 0) {
        printf("Successfully removed kazari\n");
        return 0;
    }
    fprintf(stderr, ERROR("Failed to remove kazari\n"));
    return 1;
}

int cli(int argc, char **argv) {
    if (argc == 1) {
        fprintf(stderr, ERROR("No options provided\n"));
        return 1;
    }
    char *exe_dir = malloc(4096);
    memset(exe_dir, 0, 4096);
    if (readlink("/proc/self/exe", exe_dir, 4096) == -1) {
        fprintf(stderr, ERROR("Failed to get executable directory\n"));
        return 1;
    }
    exe_dir = dirname(exe_dir);
    char *config_path = malloc(strlen(exe_dir) + 12);
    strcpy(config_path, exe_dir);
    strcat(config_path, "/tfm-config");
    const char *shortopts = "hI:O:suaftkwV";
    int status = 0;
    char *abspath = NULL;
    int opt;
    _Bool opt_a = 0;
    _Bool opt_s = 0;
    _Bool opt_u = 0;
    _Bool opt_f = 0;
    _Bool opt_t = 0;
    _Bool opt_k = 0;
    _Bool opt_w = 0;
    FILE *config = fopen(config_path, "r");
    if (config == NULL) {
        fprintf(stderr, ERROR("Config file not found\n"));
        return 1;
    }
    fseek(config, 0L, SEEK_END);
    char *tanabata_path = malloc(ftell(config) + 1);
    rewind(config);
    if (fgets(tanabata_path, INT32_MAX, config) == NULL) {
        fprintf(stderr, ERROR("Failed to read config file\n"));
        return 1;
    }
    while ((opt = getopt(argc, argv, shortopts)) != -1) {
        switch (opt) {
            case 'h':
                printf(
                        HIGHLIGHT("(C) Masahiko AMANO aka H1K0, 2022\n(https://github.com/H1K0/tanabata)\n\n")
                        HIGHLIGHT("Usage:\n")
                        "tfm <options>\n\n"
                        HIGHLIGHT("Options:\n")
                        HIGHLIGHT("-h")"        Print this help and exit\n"
                        HIGHLIGHT("-I <dir>")"  Initialize Tanabata database in directory <dir>\n"
                        HIGHLIGHT("-O <dir>")"  Open Tanabata database from directory <dir>\n"
                        HIGHLIGHT("-a")"        View all\n"
                        HIGHLIGHT("-s")"        Set or add\n"
                        HIGHLIGHT("-u")"        Unset or remove\n"
                        HIGHLIGHT("-f")"        File-sasa menu\n"
                        HIGHLIGHT("-t")"        Tanzaku menu\n"
                        HIGHLIGHT("-k")"        Kazari menu (can only be used with the '-s' or '-u' option)\n"
                        HIGHLIGHT("-w")"        Weed (defragment) database\n"
                        HIGHLIGHT("-V")"        Print version and exit\n"
                );
                if (tanabata_path != NULL) {
                    printf(HIGHLIGHT("\nCurrent database location: %s\n"), tanabata_path);
                } else {
                    printf(HIGHLIGHT("\nNo database connected\n"));
                }
                return 0;
            case 'V':
                printf("0.1.0-dev\n");
                return 0;
            case 'I':
                abspath = realpath(optarg, abspath);
                if (abspath == NULL) {
                    fprintf(stderr, ERROR("Invalid path\n"));
                    return 1;
                }
                status |= tanabata_init(&tanabata);
                status |= tanabata_dump(&tanabata, abspath);
                if (status == 0) {
                    config = freopen(NULL, "w", config);
                    if (config == NULL) {
                        fprintf(stderr, ERROR("Failed to write to config file\n"));
                        return 1;
                    }
                    fputs(abspath, config);
                    fclose(config);
                    printf("Successfully initialized Tanabata database\n");
                    return 0;
                }
                fprintf(stderr, ERROR("Failed to initialize Tanabata database\n"));
                return 1;
            case 'O':
                abspath = realpath(optarg, abspath);
                if (abspath == NULL) {
                    fprintf(stderr, ERROR("Invalid path\n"));
                    return 1;
                }
                if (tanabata_open(&tanabata, abspath) == 0) {
                    config = freopen(NULL, "w", config);
                    if (config == NULL) {
                        fprintf(stderr, ERROR("Failed to write to config file\n"));
                        return 1;
                    }
                    fputs(abspath, config);
                    fclose(config);
                    printf("Successfully opened Tanabata database\n");
                    return 0;
                }
                fprintf(stderr, ERROR("Failed to open Tanabata database\n"));
                return 1;
            case 'a':
                opt_a = 1;
                break;
            case 's':
                opt_s = 1;
                break;
            case 'u':
                opt_u = -1;
                break;
            case 'f':
                opt_f = 1;
                break;
            case 't':
                opt_t = 1;
                break;
            case 'k':
                opt_k = 1;
                break;
            case 'w':
                opt_w = 1;
                break;
            case '?':
                return 1;
            default:
                break;
        }
    }
    if (opt_s && opt_u) {
        opt_s = 0;
        opt_u = 0;
    }
    FILE *config = fopen(config_path, "r");
    if (config == NULL) {
        fprintf(stderr, ERROR("Config file not found\n"));
        return 1;
    }
    free(tanabata_path);
    fclose(config);
    if (opt_w) {
        if (tanabata_weed(&tanabata) != 0) {
            fprintf(stderr, ERROR("Failed to weed database\n"));
            return 1;
        }
        tanabata_save(&tanabata);
        printf("Successfully weeded database\n");
        return 0;
    }
    if (opt_a) {
        if (opt_f) {
            print_sasa_all();
            return 0;
        }
        if (opt_t) {
            print_tanzaku_all();
            return 0;
        }
    } else if (opt_s) {
        if (opt_f) {
            return menu_add_sasa();
        }
        if (opt_t) {
            return menu_add_tanzaku();
        }
        if (opt_k) {
            return menu_add_kazari();
        }
    } else if (opt_u) {
        if (opt_f) {
            return menu_rem_sasa();
        }
        if (opt_t) {
            return menu_rem_tanzaku();
        }
        if (opt_k) {
            return menu_rem_kazari();
        }
    } else {
        if (opt_f) {
            return menu_view_sasa();
        }
        if (opt_t) {
            return menu_view_tanzaku();
        }
    }
    return 0;
}
